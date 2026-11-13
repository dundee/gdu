package analyze

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/require"
)

type cancelRaceAnalyzerFactory struct {
	name string
	new  func(*testing.T) Analyzer
}

func TestCancelPreemptiveAnalyzers(t *testing.T) {
	root := createCancelRaceTree(t)

	for _, factory := range cancelRaceAnalyzerFactories() {
		t.Run(factory.name, func(t *testing.T) {
			analyzer := factory.new(t)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			dir := analyzer.AnalyzeDir(ctx, root, Options{})

			require.NotNil(t, dir)
			var children int
			for range dir.GetFiles(fs.SortByName, fs.SortAsc) {
				children++
			}
			require.Zero(t, children, "preemptive cancellation must not schedule child scans")
		})
	}
}

func TestCancelWhileAnalyzingAnalyzers(t *testing.T) {
	root := createCancelRaceTree(t)
	blockedPath := filepath.Join(root, "1")

	for _, factory := range cancelRaceAnalyzerFactories() {
		t.Run(factory.name, func(t *testing.T) {
			analyzer := factory.new(t)
			started := make(chan struct{})
			release := make(chan struct{})

			ignore := func(_ string, path string) bool {
				if path == blockedPath {
					close(started)
					<-release
				}
				return false
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan fs.Item, 1)
			go func() {
				result <- analyzer.AnalyzeDir(ctx, root, Options{IgnoreDir: ignore})
			}()

			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("analyzer did not reach the blocked root entry")
			}
			cancel()
			close(release)

			select {
			case dir := <-result:
				require.NotNil(t, dir)
				require.NotContains(t, itemNames(dir), "1")
			case <-time.After(10 * time.Second):
				t.Fatal("analyzer did not finish after cancellation")
			}
		})
	}
}

func cancelRaceAnalyzerFactories() []cancelRaceAnalyzerFactory {
	return []cancelRaceAnalyzerFactory{
		{name: "parallel", new: func(*testing.T) Analyzer { return CreateAnalyzer() }},
		{name: "sequential", new: func(*testing.T) Analyzer { return CreateSeqAnalyzer() }},
		{name: "stored", new: func(t *testing.T) Analyzer {
			return CreateStoredAnalyzer(filepath.Join(t.TempDir(), "results.badger"))
		}},
		{name: "sqlite", new: func(t *testing.T) Analyzer {
			analyzer, err := CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "results.sqlite"))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, analyzer.storage.Close()) })
			return analyzer
		}},
		{name: "top-dir", new: func(*testing.T) Analyzer { return CreateTopDirAnalyzer() }},
	}
}

func createCancelRaceTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for i := range 8 {
		top := filepath.Join(root, strconv.Itoa(i))
		require.NoError(t, os.Mkdir(top, 0o700))
		for j := range 8 {
			nested := filepath.Join(top, strconv.Itoa(j))
			require.NoError(t, os.Mkdir(nested, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(nested, "file"), []byte("data"), 0o600))
		}
	}
	return root
}
