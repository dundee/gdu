package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

// TestSnapshotDirsAreListedButNotScanned pins that every analyzer shows a nested
// btrfs subvolume as an empty zero sized entry flagged 'S' and never counts its content.
func TestSnapshotDirsAreListedButNotScanned(t *testing.T) {
	isNestedBtrfsSubvolume = func(path string, _ map[string]struct{}) bool {
		return filepath.Base(path) == "snaps"
	}
	t.Cleanup(func() { isNestedBtrfsSubvolume = nilSubvolume })

	root := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "snaps", "1"), os.ModePerm))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "snaps", "1", "big"), make([]byte, 1<<20), 0o600))
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "normal"), os.ModePerm))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "normal", "f"), []byte("hello"), 0o600))

	analyzers := append(parityAnalyzers(),
		parityAnalyzer{"parallel", func(*testing.T) common.Analyzer { return CreateAnalyzer() }},
	)
	for _, tc := range analyzers {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := tc.analyzer(t)
			analyzer.SetSkipBtrfsSubvolumes([]string{})
			got := scanWith(t, analyzer, root)

			assert.Equal(t, itemStats{size: 0, usage: 0, itemCount: 1, flag: snapshotFlag}, got["snaps"])
			assert.NotContains(t, got, filepath.Join("snaps", "1"))
			assert.Less(t, got["."].size, int64(1<<20), "snapshot content must not be counted")
		})
	}
}

func nilSubvolume(_ string, _ map[string]struct{}) bool { return false }

// TestSnapshotDirsTopDirAnalyzer covers the summary analyzer, whose result cannot be walked like a full tree.
func TestSnapshotDirsTopDirAnalyzer(t *testing.T) {
	isNestedBtrfsSubvolume = func(path string, _ map[string]struct{}) bool {
		return filepath.Base(path) == "snaps"
	}
	t.Cleanup(func() { isNestedBtrfsSubvolume = nilSubvolume })

	root := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "snaps"), os.ModePerm))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "snaps", "big"), make([]byte, 1<<20), 0o600))

	analyzer := CreateTopDirAnalyzer()
	analyzer.SetSkipBtrfsSubvolumes([]string{})
	dir := analyzer.AnalyzeDir(root, func(_, _ string) bool { return false }, func(_ string) bool { return false })
	analyzer.GetDone().Wait()

	files := dir.(*SimpleDir).Files
	assert.Len(t, files, 1)
	assert.Equal(t, SimpleFile{Name: "snaps", Flag: snapshotFlag, ItemCount: 1, IsDir: true}, files[0])
}

func TestAttachSnapshotScopeStaysOutOfParentTotals(t *testing.T) {
	root := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "snaps", "2"), os.ModePerm))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "snaps", "2", "f"), make([]byte, 1<<20), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "plain"), []byte("hello"), 0o600))

	parent := &Dir{File: &File{Name: filepath.Base(root)}, BasePath: filepath.Dir(root), ItemCount: 1}
	placeholder := snapshotPlaceholder("snaps", root, parent)
	parent.AddFile(placeholder)
	parent.AddFile(&File{Name: "plain", Size: 5, Usage: 4096, Parent: parent})
	parent.UpdateStats(make(fs.HardLinkedItems))
	usageBefore := parent.GetUsage()

	analyzer := CreateSubvolumeAnalyzer()
	scanned := analyzer.AnalyzeDir(filepath.Join(root, "snaps"), func(_, _ string) bool { return false }, nil)
	analyzer.GetDone().Wait()
	assert.NoError(t, AttachSnapshotScope(placeholder, scanned))

	parent.UpdateStats(make(fs.HardLinkedItems))
	assert.Equal(t, usageBefore, parent.GetUsage(), "the scope must not count for the rest of the disk")
	assert.Equal(t, snapshotFlag, scanned.GetFlag())
	assert.Equal(t, filepath.Join(root, "snaps"), scanned.GetPath())
	assert.GreaterOrEqual(t, scanned.GetSize(), int64(1<<20), "the scope reports its own size")
	assert.Equal(t, "2", slices.Collect(scanned.GetFiles(fs.SortByName, fs.SortAsc))[0].GetName())
	assert.Error(t, AttachSnapshotScope(placeholder, scanned), "the placeholder is gone after attaching")
}
