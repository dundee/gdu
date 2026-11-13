package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
		{name: "sequential", analyzer: CreateSeqAnalyzer()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := scanUntilCancelled(t, test.analyzer, root)

			assert.Contains(t, itemNames(dir), "a-file")
			assert.NotContains(t, itemNames(dir), "b-dir")
		})
	}
}

func TestStorageBackedAnalyzersCancelKeepsPartialResults(t *testing.T) {
	root := createCancellationTree(t)

	stored := CreateStoredAnalyzer(filepath.Join(t.TempDir(), "results.badger"))
	dir := scanUntilCancelled(t, stored, root)
	assert.Contains(t, itemNames(dir), "a-file")
	assert.NotContains(t, itemNames(dir), "b-dir")

	sqliteAnalyzer, err := CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "results.sqlite"))
	assert.NoError(t, err)
	defer sqliteAnalyzer.storage.Close()
	dir = scanUntilCancelled(t, sqliteAnalyzer, root)
	assert.Contains(t, itemNames(dir), "a-file")
	assert.NotContains(t, itemNames(dir), "b-dir")
}

func TestAnalyzeDirHonorsPreemptiveCancellation(t *testing.T) {
	analyzer := CreateSeqAnalyzer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dir := analyzer.AnalyzeDir(ctx, createCancellationTree(t), common.ScanOptions{})

	assert.NotContains(t, itemNames(dir), "a-file")
	assert.NotContains(t, itemNames(dir), "b-dir")
}

// A scan is cancelled by its own context only.
func TestScanAfterCancelledScanIsComplete(t *testing.T) {
	analyzer := CreateSeqAnalyzer()
	root := createCancellationTree(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	analyzer.AnalyzeDir(ctx, root, common.ScanOptions{})

	dir := analyzer.AnalyzeDir(context.Background(), root, common.ScanOptions{})

	assert.Contains(t, itemNames(dir), "a-file")
	assert.Contains(t, itemNames(dir), "b-dir")
}

func createCancellationTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(root, "a-file"), []byte("complete"), 0o600))
	assert.NoError(t, os.Mkdir(filepath.Join(root, "b-dir"), 0o700))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "b-dir", "nested"), []byte("pending"), 0o600))
	return root
}

// scanUntilCancelled scans root, cancelling the scan when it meets b-dir.
func scanUntilCancelled(t *testing.T, analyzer common.Analyzer, root string) fs.Item {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return analyzer.AnalyzeDir(ctx, root, common.ScanOptions{IgnoreDir: func(name, _ string) bool {
		if name == "b-dir" {
			cancel()
		}
		return false
	}})
}

func itemNames(dir fs.Item) []string {
	var names []string
	for item := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		names = append(names, item.GetName())
	}
	return names
}
