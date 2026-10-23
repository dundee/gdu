package analyze

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

// itemStats is the reported accounting for one directory in a scanned tree.
type itemStats struct {
	size      int64
	usage     int64
	itemCount int64
	flag      rune
}

// createEmptyDirTree builds a tree that exercises every branch of gdu's
// empty-directory rule: a leaf empty dir, a dir holding only an empty dir, a
// chain of them, and a dir with real content to keep the normal path covered.
func createEmptyDirTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"empty", "onlyempty/sub", "deep/a/b/c", "withfile"} {
		assert.NoError(t, os.MkdirAll(filepath.Join(root, dir), os.ModePerm))
	}
	assert.NoError(t, os.WriteFile(filepath.Join(root, "withfile", "f"), []byte("hello"), 0o600))

	return root
}

// collectStats walks a scanned tree and records the stats of every directory,
// keyed by path relative to the scan root.
func collectStats(root fs.Item) map[string]itemStats {
	stats := make(map[string]itemStats)
	base := root.GetPath()

	var walk func(item fs.Item)
	walk = func(item fs.Item) {
		key, err := filepath.Rel(base, item.GetPath())
		if err != nil {
			key = item.GetPath()
		}
		stats[key] = itemStats{
			size:      item.GetSize(),
			usage:     item.GetUsage(),
			itemCount: item.GetItemCount(),
			flag:      item.GetFlag(),
		}
		for child := range item.GetFiles(fs.SortByName, fs.SortAsc) {
			if child.IsDir() {
				walk(child)
			}
		}
	}
	walk(root)

	return stats
}

// usageOf returns the disk usage stat reports for a path, which is what gdu
// must count for it: 0 for an XFS inline directory, the real block count
// elsewhere.
func usageOf(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Lstat(path)
	assert.NoError(t, err)
	usage, _ := getPlatformSpecificUsageAndMli(info)
	return usage
}

// scanWith runs an unfiltered scan. The nil file type filter is significant:
// per common.UI.CreateFileTypeFilter, nil is how "no filtering" is expressed,
// and analyzers that finalize stats during the scan read it to pick between
// UpdateStats and UpdateStatsWithFileFiltering accounting.
func scanWith(t *testing.T, analyzer common.Analyzer, root string) map[string]itemStats {
	t.Helper()

	dir := analyzer.AnalyzeDir(root, func(_, _ string) bool { return false }, nil)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	return collectStats(dir)
}

// scanFilteringAllFiles runs a scan whose type filter rejects every file, so
// the whole tree becomes empty directories and the collapse branch of
// resolveDirStats applies.
func scanFilteringAllFiles(
	t *testing.T, analyzer common.Analyzer, root string,
) map[string]itemStats {
	t.Helper()

	dir := analyzer.AnalyzeDir(
		root, func(_, _ string) bool { return false }, func(_ string) bool { return true },
	)
	analyzer.GetDone().Wait()
	dir.UpdateStatsWithFileFiltering(make(fs.HardLinkedItems))

	return collectStats(dir)
}

// TestEmptyDirStatsAcrossAnalyzers pins the empty-directory rule: a directory
// with no counted children reports EmptyDirSize apparent size and zero disk
// usage, and a tree of nothing but empty directories therefore uses no disk at
// all. Every analyzer must agree, so the numbers gdu prints cannot depend on
// whether --db was passed.
func TestEmptyDirStatsAcrossAnalyzers(t *testing.T) {
	root := createEmptyDirTree(t)

	// Disk usage depends on the filesystem block size, so take every expected
	// value from stat. A directory is charged for the blocks it actually
	// occupies -- 0 for an XFS inline directory, a real block elsewhere
	// (issue #393) -- and a directory's usage also carries its children's.
	fileUsage := usageOf(t, filepath.Join(root, "withfile", "f"))
	emptyUsage := usageOf(t, filepath.Join(root, "empty"))
	subEmptyUsage := usageOf(t, filepath.Join(root, "onlyempty", "sub"))
	onlyemptyUsage := subEmptyUsage + usageOf(t, filepath.Join(root, "onlyempty"))
	leafUsage := usageOf(t, filepath.Join(root, "deep", "a", "b", "c"))
	deepBUsage := leafUsage + usageOf(t, filepath.Join(root, "deep", "a", "b"))
	deepAUsage := deepBUsage + usageOf(t, filepath.Join(root, "deep", "a"))
	deepUsage := deepAUsage + usageOf(t, filepath.Join(root, "deep"))

	want := map[string]itemStats{
		".": {size: 1541, usage: fileUsage + deepUsage + onlyemptyUsage + emptyUsage +
			usageOf(t, filepath.Join(root, "withfile")) + usageOf(t, root), itemCount: 10, flag: ' '},
		"empty":     {size: EmptyDirSize, usage: emptyUsage, itemCount: 1, flag: 'e'},
		"onlyempty": {size: EmptyDirSize, usage: onlyemptyUsage, itemCount: 2, flag: ' '},
		"onlyempty/sub": {
			size: EmptyDirSize, usage: subEmptyUsage, itemCount: 1, flag: 'e',
		},
		"deep":       {size: EmptyDirSize, usage: deepUsage, itemCount: 4, flag: ' '},
		"deep/a":     {size: EmptyDirSize, usage: deepAUsage, itemCount: 3, flag: ' '},
		"deep/a/b":   {size: EmptyDirSize, usage: deepBUsage, itemCount: 2, flag: ' '},
		"deep/a/b/c": {size: EmptyDirSize, usage: leafUsage, itemCount: 1, flag: 'e'},
	}

	parallel := scanWith(t, CreateAnalyzer(), root)

	// The reference values above are asserted against the in-memory analyzer
	// first, so a genuine change in the rule fails here rather than silently
	// re-baselining every analyzer at once.
	for path, expected := range want {
		assert.Equal(t, expected, parallel[path], "parallel analyzer at %q", path)
	}
	assert.Equal(t, emptyUsage, parallel["empty"].usage,
		"an empty dir must cost its own blocks, and nothing else")
	assert.Equal(t, deepUsage, parallel["deep"].usage,
		"a tree of empty dirs must cost the blocks those dirs actually occupy")

	// Under file filtering the whole tree collapses, because every directory
	// ends up holding nothing but empty directories. The root reports a single
	// item and costs only the blocks the root itself occupies.
	wantFiltered := scanFilteringAllFiles(t, CreateAnalyzer(), root)
	assert.Equal(t, int64(1), wantFiltered["."].itemCount)
	assert.Equal(t, usageOf(t, root), wantFiltered["."].usage)

	for _, tc := range parityAnalyzers() {
		t.Run(tc.name, func(t *testing.T) {
			got := scanWith(t, tc.analyzer(t), root)
			for path, expected := range want {
				assert.Equal(t, expected, got[path], "%s analyzer at %q", tc.name, path)
			}
		})

		t.Run(tc.name+" filtering files", func(t *testing.T) {
			got := scanFilteringAllFiles(t, tc.analyzer(t), root)
			for path, expected := range wantFiltered {
				assert.Equal(t, expected, got[path], "%s analyzer at %q", tc.name, path)
			}
		})
	}
}

type parityAnalyzer struct {
	name     string
	analyzer func(t *testing.T) common.Analyzer
}

func parityAnalyzers() []parityAnalyzer {
	return []parityAnalyzer{
		{
			name:     "sequential",
			analyzer: func(*testing.T) common.Analyzer { return CreateSeqAnalyzer() },
		},
		{
			name:     "stable order",
			analyzer: func(*testing.T) common.Analyzer { return CreateStableOrderAnalyzer() },
		},
		{
			name: "stored (badger)",
			analyzer: func(t *testing.T) common.Analyzer {
				return CreateStoredAnalyzer(filepath.Join(t.TempDir(), "badger"))
			},
		},
		{
			name: "sqlite",
			analyzer: func(t *testing.T) common.Analyzer {
				analyzer, err := CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "test.db"))
				assert.NoError(t, err)
				t.Cleanup(func() { _ = analyzer.storage.Close() })
				return analyzer
			},
		},
	}
}
