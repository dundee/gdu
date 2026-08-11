package analyze

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzersCancelKeepsPartialResults(t *testing.T) {
	root := createCancellationTree(t)

	tests := []struct {
		name     string
		analyzer common.Analyzer
	}{
		{name: "parallel", analyzer: CreateAnalyzer()},
		{name: "stable parallel", analyzer: CreateStableOrderAnalyzer()},
		{name: "sequential", analyzer: CreateSeqAnalyzer()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := scanUntilCancelled(t, test.analyzer, root)

			assert.True(t, test.analyzer.(interface{ IsCancelled() bool }).IsCancelled())
			assert.Contains(t, itemNames(dir), "a-file")
			assert.NotContains(t, itemNames(dir), "b-dir")
		})
	}
}

func TestParallelAnalyzersCancelQueuedDirectories(t *testing.T) {
	root := createCancellationTree(t)
	tests := []struct {
		name    string
		factory func() common.Analyzer
	}{
		{name: "parallel", factory: func() common.Analyzer { return CreateAnalyzer() }},
		{name: "stable parallel", factory: func() common.Analyzer { return CreateStableOrderAnalyzer() }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initialSlots := len(concurrencyLimit)
			for len(concurrencyLimit) < cap(concurrencyLimit) {
				concurrencyLimit <- struct{}{}
			}
			defer func() {
				for len(concurrencyLimit) > initialSlots {
					<-concurrencyLimit
				}
			}()

			analyzer := test.factory()
			result := make(chan fs.Item, 1)
			go func() {
				result <- analyzer.AnalyzeDir(root, func(_, _ string) bool { return false }, nil)
			}()

			assert.Eventually(t, func() bool {
				return analyzer.GetProgress().CurrentItemName == root
			}, time.Second, time.Millisecond)
			analyzer.Cancel()
			<-concurrencyLimit

			select {
			case dir := <-result:
				assert.NotContains(t, itemNames(dir), "b-dir")
			case <-time.After(time.Second):
				t.Fatal("analyzer did not discard queued directory after cancellation")
			}
		})
	}
}

func TestStorageBackedAnalyzersCancelKeepsPartialResults(t *testing.T) {
	root := createCancellationTree(t)

	stored := CreateStoredAnalyzer(filepath.Join(t.TempDir(), "results.badger"))
	dir := scanUntilCancelled(t, stored, root)
	assert.True(t, stored.IsCancelled())
	assert.Contains(t, itemNames(dir), "a-file")
	assert.NotContains(t, itemNames(dir), "b-dir")

	sqliteAnalyzer, err := CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "results.sqlite"))
	assert.NoError(t, err)
	defer sqliteAnalyzer.storage.Close()
	dir = scanUntilCancelled(t, sqliteAnalyzer, root)
	assert.True(t, sqliteAnalyzer.IsCancelled())
	assert.Contains(t, itemNames(dir), "a-file")
	assert.NotContains(t, itemNames(dir), "b-dir")
}

func TestAnalyzeDirHonorsPreemptiveCancellation(t *testing.T) {
	analyzer := CreateSeqAnalyzer()
	analyzer.Cancel()

	dir := analyzer.AnalyzeDir(createCancellationTree(t), func(_, _ string) bool { return false }, nil)

	assert.True(t, analyzer.IsCancelled())
	assert.NotContains(t, itemNames(dir), "a-file")
	assert.NotContains(t, itemNames(dir), "b-dir")
}

func TestResetProgressClearsPreviousCancellation(t *testing.T) {
	analyzer := CreateSeqAnalyzer()
	analyzer.Cancel()
	analyzer.ResetProgress()

	dir := analyzer.AnalyzeDir(createCancellationTree(t), func(_, _ string) bool { return false }, nil)

	assert.False(t, analyzer.IsCancelled())
	assert.Contains(t, itemNames(dir), "a-file")
	assert.Contains(t, itemNames(dir), "b-dir")
}

func TestTopDirAnalyzerCancellationStopsActiveSubdirectory(t *testing.T) {
	root := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(root, "unscanned"), []byte("data"), 0o600))

	analyzer := CreateTopDirAnalyzer()
	t.Cleanup(analyzer.progressTicker.Stop)
	analyzer.Cancel()
	result := &TopDir{}

	analyzer.processSubDir(root, result)

	size, usage, itemCount := result.GetUsage()
	assert.Zero(t, size)
	assert.Zero(t, usage)
	assert.Zero(t, itemCount)
}

func createCancellationTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(root, "a-file"), []byte("complete"), 0o600))
	assert.NoError(t, os.Mkdir(filepath.Join(root, "b-dir"), 0o700))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "b-dir", "nested"), []byte("pending"), 0o600))
	return root
}

func scanUntilCancelled(t *testing.T, analyzer common.Analyzer, root string) fs.Item {
	t.Helper()
	return analyzer.AnalyzeDir(root, func(name, _ string) bool {
		if name == "b-dir" {
			analyzer.Cancel()
		}
		return false
	}, nil)
}

func itemNames(dir fs.Item) []string {
	var names []string
	for item := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		names = append(names, item.GetName())
	}
	return names
}
