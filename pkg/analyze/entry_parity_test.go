package analyze

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// entryStats is the reported accounting for one item of a scanned tree.
type entryStats struct {
	dir                bool
	size, usage, count int64
	flag               rune
}

// collectEntries walks a scanned tree and records the stats of every item,
// keyed by path relative to the scan root.
func collectEntries(root fs.Item) map[string]entryStats {
	stats := make(map[string]entryStats)
	base := root.GetPath()

	var walk func(item fs.Item)
	walk = func(item fs.Item) {
		key, err := filepath.Rel(base, item.GetPath())
		if err != nil {
			key = item.GetPath()
		}
		stats[key] = entryStats{item.IsDir(), item.GetSize(), item.GetUsage(), item.GetItemCount(), item.GetFlag()}
		if item.IsDir() {
			for child := range item.GetFiles(fs.SortByName, fs.SortAsc) {
				walk(child)
			}
		}
	}
	walk(root)
	return stats
}

// entryParityAnalyzers returns every analyzer that builds a full tree, the
// in-memory one first.
func entryParityAnalyzers() []parityAnalyzer {
	return append([]parityAnalyzer{{
		name:     "parallel",
		analyzer: func(*testing.T) common.Analyzer { return CreateAnalyzer() },
	}}, parityAnalyzers()...)
}

// scanConfigured scans root with an analyzer set up by configure and computes
// its totals the way the session does.
func scanConfigured(
	t *testing.T, analyzer common.Analyzer, root string, configure func(common.Analyzer),
) fs.Item {
	t.Helper()
	if configure != nil {
		configure(analyzer)
	}
	done := make(chan fs.Item, 1)
	go func() {
		done <- analyzer.AnalyzeDir(root, func(_, _ string) bool { return false }, nil)
	}()
	var dir fs.Item
	select {
	case dir = <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the scan did not finish")
	}
	analyzer.GetDone().Wait()
	dir.UpdateStatsWithFileFiltering(make(fs.HardLinkedItems))
	return dir
}

// assertEntryParity scans root with every analyzer configured alike and
// checks each reports the tree the in-memory analyzer reports.
func assertEntryParity(t *testing.T, root string, configure func(common.Analyzer)) map[string]entryStats {
	t.Helper()
	analyzers := entryParityAnalyzers()
	want := collectEntries(scanConfigured(t, analyzers[0].analyzer(t), root, configure))
	for _, tc := range analyzers[1:] {
		t.Run(tc.name, func(t *testing.T) {
			got := collectEntries(scanConfigured(t, tc.analyzer(t), root, configure))
			assert.Equal(t, want, got)
		})
	}
	t.Run("top-dir", func(t *testing.T) {
		got := scanConfigured(t, CreateTopDirAnalyzer(), root, configure)
		assert.Equal(t, want["."].size, got.GetSize(), "size at the root")
		assert.Equal(t, want["."].usage, got.GetUsage(), "usage at the root")
		for child := range got.GetFiles(fs.SortByName, fs.SortAsc) {
			w := want[child.GetName()]
			assert.Equal(t, w.size, child.GetSize(), "size of %s", child.GetName())
			assert.Equal(t, w.usage, child.GetUsage(), "usage of %s", child.GetName())
			assert.Equal(t, w.flag, child.GetFlag(), "flag of %s", child.GetName())
		}
	})
	return want
}

func writeSized(t *testing.T, path string, size int, mtime time.Time) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o600))
	if !mtime.IsZero() {
		require.NoError(t, os.Chtimes(path, mtime, mtime))
	}
}

// With symlinks followed, the time filter applies to the file a link points
// to, as it does to the size counted for the link.
func TestFollowedSymlinkTimeFilterParity(t *testing.T) {
	tmp := t.TempDir()
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	target := filepath.Join(tmp, "outside", "old")
	writeSized(t, target, 10000, old)
	root := filepath.Join(tmp, "root")
	writeSized(t, filepath.Join(root, "sub", "new"), 300, time.Time{})
	require.NoError(t, os.Symlink(target, filepath.Join(root, "sub", "link")))

	want := assertEntryParity(t, root, func(a common.Analyzer) {
		a.SetFollowSymlinks(true)
		a.SetTimeFilter(func(mtime time.Time) bool { return mtime.Before(old.Add(time.Hour)) })
	})

	assert.Equal(t, int64(10000), want["sub/link"].size)
	assert.NotContains(t, want, "sub/new")
}

// An entry that cannot be read marks its directory as read with errors.
func TestUnreadableEntryFlagParity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	writeSized(t, filepath.Join(root, "sub", "file"), 300, time.Time{})
	require.NoError(t, os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "sub", "dangling")))

	want := assertEntryParity(t, root, func(a common.Analyzer) { a.SetFollowSymlinks(true) })

	assert.Equal(t, '!', want["sub"].flag)
	assert.Equal(t, '.', want["."].flag)
}
