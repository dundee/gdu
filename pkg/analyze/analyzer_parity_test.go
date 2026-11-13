package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

// usageOf returns the disk usage stat reports for a path itself.
func usageOf(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Lstat(path)
	assert.NoError(t, err)
	usage, _ := fileStats(info, path)
	return usage
}

// scanWith runs an unfiltered scan. The nil file type filter is significant:
// per common.UI.CreateFileTypeFilter, nil is how "no filtering" is expressed,
// and analyzers that finalize stats during the scan read it to pick between
// UpdateStats and UpdateStatsWithFileFiltering accounting.
func scanWith(t *testing.T, analyzer Analyzer, root string) map[string]itemStats {
	t.Helper()

	dir := analyzer.AnalyzeDir(context.Background(), root, Options{})
	dir.UpdateStats(false)

	return collectStats(dir)
}

// scanFilteringAllFiles runs a scan whose type filter rejects every file, so
// the whole tree becomes empty directories and the collapse branch of
// resolveDirStats applies.
func scanFilteringAllFiles(
	t *testing.T, analyzer Analyzer, root string,
) map[string]itemStats {
	t.Helper()

	dir := analyzer.AnalyzeDir(context.Background(), root, Options{
		IgnoreFile: func(string) bool { return true },
	})
	dir.UpdateStats(true)

	return collectStats(dir)
}

// TestEmptyDirStatsAcrossAnalyzers pins the empty-directory rule: a directory
// with no counted children reports EmptyDirSize apparent size, and every
// directory is charged the disk usage of its own blocks. Every analyzer must
// agree, so the numbers gdu prints cannot depend on whether --db was passed.
func TestEmptyDirStatsAcrossAnalyzers(t *testing.T) {
	root := createEmptyDirTree(t)

	// disk usage depends on the filesystem, so take every expected value from stat
	own := func(rel string) int64 { return usageOf(t, filepath.Join(root, rel)) }
	fileUsage := own("withfile/f")
	leafUsage := own("deep/a/b/c")
	deepBUsage := leafUsage + own("deep/a/b")
	deepAUsage := deepBUsage + own("deep/a")
	deepUsage := deepAUsage + own("deep")
	subUsage := own("onlyempty/sub")
	onlyemptyUsage := subUsage + own("onlyempty")
	dirsUsage := deepUsage + onlyemptyUsage + own("empty") + own("withfile") + own(".")

	want := map[string]itemStats{
		".":             {size: 1541, usage: dirsUsage + fileUsage, itemCount: 10, flag: ' '},
		"empty":         {size: EmptyDirSize, usage: own("empty"), itemCount: 1, flag: 'e'},
		"onlyempty":     {size: EmptyDirSize, usage: onlyemptyUsage, itemCount: 2, flag: ' '},
		"onlyempty/sub": {size: EmptyDirSize, usage: subUsage, itemCount: 1, flag: 'e'},
		"deep":          {size: EmptyDirSize, usage: deepUsage, itemCount: 4, flag: ' '},
		"deep/a":        {size: EmptyDirSize, usage: deepAUsage, itemCount: 3, flag: ' '},
		"deep/a/b":      {size: EmptyDirSize, usage: deepBUsage, itemCount: 2, flag: ' '},
		"deep/a/b/c":    {size: EmptyDirSize, usage: leafUsage, itemCount: 1, flag: 'e'},
	}

	parallel := scanWith(t, CreateAnalyzer(), root)

	// The reference values above are asserted against the in-memory analyzer
	// first, so a genuine change in the rule fails here rather than silently
	// re-baselining every analyzer at once.
	for path, expected := range want {
		assert.Equal(t, expected, parallel[path], "parallel analyzer at %q", path)
	}

	// Under file filtering the whole tree collapses to a single item, because
	// every directory ends up holding nothing but empty directories. The
	// directories still occupy their blocks.
	wantFiltered := scanFilteringAllFiles(t, CreateAnalyzer(), root)
	assert.Equal(t, int64(1), wantFiltered["."].itemCount)
	assert.Equal(t, dirsUsage, wantFiltered["."].usage)

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

// TestTopDirAnalyzerUsageParity checks the disk usage reported by the top-dir
// analyzer against the in-memory one. It reports only the scan root and its
// immediate entries, so only those are compared.
func TestTopDirAnalyzerUsageParity(t *testing.T) {
	root := createEmptyDirTree(t)
	ignoreAllFiles := func(string) bool { return true }

	for _, tc := range []struct {
		name       string
		fileFilter ShouldFileBeIgnored
		want       map[string]itemStats
	}{
		{name: "unfiltered", want: scanWith(t, CreateAnalyzer(), root)},
		{
			name:       "filtering files",
			fileFilter: ignoreAllFiles,
			want:       scanFilteringAllFiles(t, CreateAnalyzer(), root),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := CreateTopDirAnalyzer()
			dir := analyzer.AnalyzeDir(context.Background(), root, Options{IgnoreFile: tc.fileFilter})
			dir.UpdateStats(false)

			assert.Equal(t, tc.want["."].usage, dir.GetUsage(), "top-dir analyzer at root")
			for child := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
				if child.IsDir() {
					assert.Equal(t, tc.want[child.GetName()].usage, child.GetUsage(),
						"top-dir analyzer at %q", child.GetName())
				}
			}
		})
	}
}

type parityAnalyzer struct {
	name     string
	analyzer func(t *testing.T) Analyzer
}

func parityAnalyzers() []parityAnalyzer {
	return []parityAnalyzer{
		{
			name:     "sequential",
			analyzer: func(*testing.T) Analyzer { return CreateSeqAnalyzer() },
		},
		{
			name: "stored (badger)",
			analyzer: func(t *testing.T) Analyzer {
				return createStoredAnalyzer(filepath.Join(t.TempDir(), "badger"))
			},
		},
		{
			name: "sqlite",
			analyzer: func(t *testing.T) Analyzer {
				analyzer, err := createSqliteAnalyzer(filepath.Join(t.TempDir(), "test.db"))
				assert.NoError(t, err)
				t.Cleanup(func() { _ = analyzer.storage.Close() })
				return analyzer
			},
		},
	}
}
