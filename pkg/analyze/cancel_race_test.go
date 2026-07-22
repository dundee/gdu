package analyze

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/require"
)

type cancelRaceAnalyzer interface {
	common.Analyzer
	IsCancelled() bool
}

type cancelRaceAnalyzerFactory struct {
	name string
	new  func(*testing.T) cancelRaceAnalyzer
}

func TestCancelPreemptiveAnalyzers(t *testing.T) {
	root := createCancelRaceTree(t)

	for _, factory := range cancelRaceAnalyzerFactories() {
		t.Run(factory.name, func(t *testing.T) {
			analyzer := factory.new(t)
			analyzer.Cancel()

			dir := analyzer.AnalyzeDir(root, func(string, string) bool { return false }, nil)

			require.NotNil(t, dir)
			require.True(t, analyzer.IsCancelled())
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

			result := make(chan fs.Item, 1)
			go func() {
				result <- analyzer.AnalyzeDir(root, ignore, nil)
			}()

			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("analyzer did not reach the blocked root entry")
			}
			analyzer.Cancel()
			close(release)

			select {
			case dir := <-result:
				require.NotNil(t, dir)
				require.True(t, analyzer.IsCancelled())
				require.NotContains(t, itemNames(dir), "1")
			case <-time.After(10 * time.Second):
				t.Fatal("analyzer did not finish after cancellation")
			}
		})
	}
}

func cancelRaceAnalyzerFactories() []cancelRaceAnalyzerFactory {
	return []cancelRaceAnalyzerFactory{
		{name: "parallel", new: func(*testing.T) cancelRaceAnalyzer { return CreateAnalyzer() }},
		{name: "stable", new: func(*testing.T) cancelRaceAnalyzer { return CreateStableOrderAnalyzer() }},
		{name: "sequential", new: func(*testing.T) cancelRaceAnalyzer { return CreateSeqAnalyzer() }},
		{name: "stored", new: func(t *testing.T) cancelRaceAnalyzer {
			return CreateStoredAnalyzer(filepath.Join(t.TempDir(), "results.badger"))
		}},
		{name: "sqlite", new: func(t *testing.T) cancelRaceAnalyzer {
			analyzer, err := CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "results.sqlite"))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, analyzer.storage.Close()) })
			return analyzer
		}},
		{name: "top-dir", new: func(*testing.T) cancelRaceAnalyzer { return CreateTopDirAnalyzer() }},
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
