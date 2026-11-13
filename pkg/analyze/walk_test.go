package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// createDatedTree builds a tree whose directories have mtimes older than the
// files in them, an empty directory, and a directory holding only an empty
// directory.
func createDatedTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	files := map[string]time.Time{
		"top-file":            time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		"sub/a":               time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
		"sub/deeper/b":        time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC),
		"other/c":             time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC),
		"other/inner/deeper/": {},
	}
	for path, mtime := range files {
		full := filepath.Join(root, path)
		if path[len(path)-1] == '/' {
			require.NoError(t, os.MkdirAll(full, 0o755))
			continue
		}
		writeSized(t, full, 1000, mtime)
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "empty"), 0o755))
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, dir := range []string{"sub/deeper", "sub", "other/inner/deeper", "other/inner", "other", "empty", "."} {
		require.NoError(t, os.Chtimes(filepath.Join(root, dir), old, old))
	}
	return root
}

// A directory reports the newest mtime in it, and the empty-directory rule
// applies alike, whether the totals are computed after the scan or during it.
func TestDirectoryTotalsParity(t *testing.T) {
	root := createDatedTree(t)

	want := assertEntryParity(t, root, nil)

	assert.Equal(t, int64(2015), int64(time.Unix(want["sub"].mtime, 0).UTC().Year()))
	assert.Equal(t, EmptyDirSize, want["empty"].size)
}

func TestFilteredDirectoryTotalsParity(t *testing.T) {
	root := createDatedTree(t)

	want := assertFilteredParity(t, root, nil, func(string) bool { return true })

	assert.Equal(t, int64(1), want["sub"].count, "a directory left with no files collapses")
}

// The preview shows the tree a scan has built so far, while it is scanning.
func TestPreviewDuringScan(t *testing.T) {
	root := createDatedTree(t)
	blocked := filepath.Join(root, "sub")

	for _, tc := range []struct {
		name     string
		analyzer interface {
			Analyzer
			GetCurrentDir() fs.Item
		}
	}{
		{"parallel", CreateAnalyzer()},
		{"sequential", CreateSeqAnalyzer()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reached := make(chan struct{})
			release := make(chan struct{})
			ignore := func(_, path string) bool {
				if path == blocked {
					close(reached)
					<-release
				}
				return false
			}
			done := make(chan fs.Item, 1)
			go func() {
				done <- tc.analyzer.AnalyzeDir(context.Background(), root, Options{IgnoreDir: ignore})
			}()

			<-reached
			preview := tc.analyzer.GetCurrentDir()
			close(release)
			<-done

			require.NotNil(t, preview, "the preview is there during the scan")
			assert.Equal(t, root, preview.GetPath())
		})
	}
}

// The number of directories read at once follows GOMAXPROCS as it is when the
// scan starts, which --max-cores sets after the program has started.
func TestScanConcurrencyFollowsGOMAXPROCS(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	for i := range 40 {
		require.NoError(t, os.MkdirAll(filepath.Join(root, fmt.Sprintf("d%02d", i), "a", "b"), 0o755))
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	for _, tc := range treeAnalyzersWithTopDir() {
		t.Run(tc.name, func(t *testing.T) {
			var active, most atomic.Int32
			ignore := func(string, string) bool {
				now := active.Add(1)
				for {
					seen := most.Load()
					if now <= seen || most.CompareAndSwap(seen, now) {
						break
					}
				}
				time.Sleep(time.Millisecond)
				active.Add(-1)
				return false
			}

			analyzer := tc.analyzer(t)
			analyzer.AnalyzeDir(context.Background(), root, Options{IgnoreDir: ignore})

			// the goroutine walking the root, and two per CPU
			assert.LessOrEqual(t, most.Load(), int32(3))
		})
	}
}

// Progress counts the items the tree ends up holding.
func TestProgressCountsTheItemsOfTheTree(t *testing.T) {
	root := createDatedTree(t)

	for _, tc := range treeAnalyzersWithTopDir() {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := tc.analyzer(t)
			dir := scanConfigured(t, analyzer, root, nil)

			assert.Equal(t, dir.GetItemCount(), analyzer.GetProgress().ItemCount)
		})
	}
}

func treeAnalyzersWithTopDir() []parityAnalyzer {
	return append(entryParityAnalyzers(), parityAnalyzer{
		name:     "top-dir",
		analyzer: func(*testing.T) Analyzer { return CreateTopDirAnalyzer() },
	})
}
