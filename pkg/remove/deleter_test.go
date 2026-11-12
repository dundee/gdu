package remove

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// makeScanRoot creates root/dir/{a,b/c}, root/file and root/sibling, and
// returns root.
func makeScanRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "scan")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dir", "b"), 0o755))
	writeFile(t, filepath.Join(root, "dir", "a"), "aaaa")
	writeFile(t, filepath.Join(root, "dir", "b", "c"), "cc")
	writeFile(t, filepath.Join(root, "file"), "file")
	writeFile(t, filepath.Join(root, "sibling"), "sibling")
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func scanTree(t *testing.T, root string, archives bool) fs.Item {
	t.Helper()
	analyzer := analyze.CreateAnalyzer()
	analyzer.SetArchiveBrowsing(archives)
	tree := analyzer.AnalyzeDir(root, func(string, string) bool { return false }, nil)
	tree.UpdateStats(make(fs.HardLinkedItems))
	return tree
}

func child(t *testing.T, dir fs.Item, name string) fs.Item {
	t.Helper()
	for entry := range dir.GetFilesLocked(fs.SortByName, fs.SortAsc) {
		if entry.GetName() == name {
			return entry
		}
	}
	t.Fatalf("%s has no entry %q", dir.GetPath(), name)
	return nil
}

func hasChild(dir fs.Item, name string) bool {
	for entry := range dir.GetFilesLocked(fs.SortByName, fs.SortAsc) {
		if entry.GetName() == name {
			return true
		}
	}
	return false
}

func TestDeleterRemovesItemsInsideRoot(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		root := makeScanRoot(t)
		tree := scanTree(t, root, false)
		var opts []Option
		if parallel {
			opts = append(opts, WithParallel())
		}
		deleter := NewDeleter([]string{root}, opts...)

		require.NoError(t, deleter.Delete(tree, child(t, tree, "dir"), ModeRemove))
		require.NoError(t, deleter.Delete(tree, child(t, tree, "file"), ModeRemove))

		assert.NoDirExists(t, filepath.Join(root, "dir"))
		assert.NoFileExists(t, filepath.Join(root, "file"))
		assert.FileExists(t, filepath.Join(root, "sibling"))
		assert.False(t, hasChild(tree, "dir"))
		assert.False(t, hasChild(tree, "file"))
		assert.True(t, hasChild(tree, "sibling"))
	}
}

func TestDeleterEmptiesFileAndDir(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)
	deleter := NewDeleter([]string{root})

	require.NoError(t, deleter.Delete(tree, child(t, tree, "file"), ModeEmpty))
	info, err := os.Stat(filepath.Join(root, "file"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
	assert.Equal(t, int64(0), child(t, tree, "file").GetSize())

	dir := child(t, tree, "dir")
	require.NoError(t, deleter.Delete(tree, dir, ModeEmpty))
	assert.DirExists(t, filepath.Join(root, "dir"))
	entries, err := os.ReadDir(filepath.Join(root, "dir"))
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.Equal(t, 0, len(entries))
	assert.False(t, hasChild(dir, "a"))
}

// Removing or emptying an item updates the totals of its directory and of
// every directory above it.
func TestDeleterUpdatesTotalsUpTheTree(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)
	dir := child(t, tree, "dir")
	a := child(t, dir, "a")
	deleter := NewDeleter([]string{root})

	dirSize, dirUsage, dirCount := dir.GetSize(), dir.GetUsage(), dir.GetItemCount()
	treeSize, treeUsage, treeCount := tree.GetSize(), tree.GetUsage(), tree.GetItemCount()
	require.NoError(t, deleter.Delete(dir, a, ModeRemove))

	assert.Equal(t, dirSize-a.GetSize(), dir.GetSize())
	assert.Equal(t, dirUsage-a.GetUsage(), dir.GetUsage())
	assert.Equal(t, dirCount-1, dir.GetItemCount())
	assert.Equal(t, treeSize-a.GetSize(), tree.GetSize())
	assert.Equal(t, treeUsage-a.GetUsage(), tree.GetUsage())
	assert.Equal(t, treeCount-1, tree.GetItemCount())

	file := child(t, tree, "file")
	treeSize, treeUsage = tree.GetSize(), tree.GetUsage()
	require.NoError(t, deleter.Delete(tree, file, ModeEmpty))

	assert.Equal(t, treeSize-file.GetSize(), tree.GetSize())
	assert.Equal(t, treeUsage-file.GetUsage(), tree.GetUsage())
	assert.Equal(t, int64(0), child(t, tree, "file").GetSize())
}

func TestDeleterTrashUsesTrashFunc(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)
	var trashed []string
	deleter := NewDeleter([]string{root}, WithTrash(func(dir, item fs.Item) error {
		trashed = append(trashed, item.GetPath())
		dir.RemoveFile(item)
		return nil
	}))

	require.NoError(t, deleter.Delete(tree, child(t, tree, "file"), ModeTrash))

	assert.Equal(t, []string{filepath.Join(root, "file")}, trashed)
	assert.False(t, hasChild(tree, "file"))
}

func TestDeleterRefusesScannedRoots(t *testing.T) {
	rootA := makeScanRoot(t)
	rootB := makeScanRoot(t)
	treeA := scanTree(t, rootA, false)
	treeB := scanTree(t, rootB, false)
	top := analyze.CreateVirtualRootDir(treeA, treeB)
	trashCalled := false
	deleter := NewDeleter(analyze.ScannedRootPaths(top), WithTrash(func(fs.Item, fs.Item) error {
		trashCalled = true
		return nil
	}))

	assert.ErrorIs(t, deleter.Delete(top, treeA, ModeRemove), ErrScannedRoot)
	assert.ErrorIs(t, deleter.Delete(top, treeA, ModeTrash), ErrScannedRoot)
	assert.False(t, trashCalled)
	assert.DirExists(t, rootA)

	// emptying keeps the root itself
	require.NoError(t, deleter.Delete(top, treeB, ModeEmpty))
	assert.DirExists(t, rootB)
	entries, err := os.ReadDir(rootB)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestDeleterRefusesItemsOutsideRoots(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)
	deleter := NewDeleter([]string{filepath.Join(root, "dir")})

	err := deleter.Delete(tree, child(t, tree, "file"), ModeRemove)

	assert.ErrorIs(t, err, ErrOutsideRoots)
	assert.ErrorIs(t, err, ErrRefused)
	assert.FileExists(t, filepath.Join(root, "file"))
}

func TestDeleterWithoutRootsRefusesEverything(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)

	err := NewDeleter(nil).Delete(tree, child(t, tree, "file"), ModeRemove)

	assert.ErrorIs(t, err, ErrOutsideRoots)
	assert.FileExists(t, filepath.Join(root, "file"))
}

// The TUI lists the parent directory as "/.." and refers to it from that row.
// Handing it in as an entry of the directory being browsed must not delete
// the parent.
func TestDeleterRefusesItemWhichIsNotAnEntryOfDir(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)
	dir := child(t, tree, "dir")
	sub := child(t, dir, "b")
	deleter := NewDeleter([]string{root})

	assert.ErrorIs(t, deleter.Delete(sub, dir, ModeRemove), ErrNotInDir)
	assert.ErrorIs(t, deleter.Delete(sub, dir, ModeEmpty), ErrNotInDir)
	assert.ErrorIs(t, deleter.Delete(sub, child(t, tree, "file"), ModeRemove), ErrNotInDir)
	assert.ErrorIs(t, deleter.Delete(nil, dir, ModeRemove), ErrNotInDir)
	assert.DirExists(t, filepath.Join(root, "dir"))
	assert.FileExists(t, filepath.Join(root, "file"))
}

// Names in an imported report are not validated by the import, and joining
// them onto the parent path would lead outside the item's directory.
func TestDeleterRefusesNamesWhichAreNotPathElements(t *testing.T) {
	root := makeScanRoot(t)
	tree := &analyze.Dir{
		File:     &analyze.File{Name: filepath.Base(root)},
		BasePath: filepath.Dir(root),
	}
	deleter := NewDeleter([]string{root})

	for _, name := range []string{"..", ".", "", "../..", "dir/b", "/etc"} {
		item := &analyze.File{Name: name, Parent: tree}
		tree.AddFile(item)
		assert.ErrorIs(t, deleter.Delete(tree, item, ModeRemove), ErrRefused, "name %q", name)
		assert.ErrorIs(t, deleter.Delete(tree, item, ModeEmpty), ErrRefused, "name %q", name)
	}
	assert.DirExists(t, filepath.Join(root, "dir", "b"))
	assert.DirExists(t, root)
}

func TestIsPathElement(t *testing.T) {
	for name, want := range map[string]bool{
		"file":    true,
		".hidden": true,
		"..foo":   true,
		"a b":     true,
		"":        false,
		".":       false,
		"..":      false,
		"a/b":     false,
		"/abs":    false,
		"nul\x00": false,
	} {
		assert.Equal(t, want, isPathElement(name), "name %q", name)
	}
}

// writeZip writes a zip archive holding the given entries.
func writeZip(t *testing.T, path string, names ...string) {
	t.Helper()
	out, err := os.Create(path)
	require.NoError(t, err)
	w := zip.NewWriter(out)
	for _, name := range names {
		entry, err := w.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte("archived"))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())
}

// An archive entry named "../victim" becomes a synthetic directory named ".."
// whose path is the directory holding the archive. Removing or emptying
// anything inside an archive must never reach the disk.
func TestDeleterNeverTouchesArchiveEntries(t *testing.T) {
	root := makeScanRoot(t)
	writeZip(t, filepath.Join(root, "evil.zip"), "../victim/f", "inner/g")

	for _, parallel := range []bool{false, true} {
		tree := scanTree(t, root, true)
		archive := child(t, tree, "evil.zip")
		require.True(t, analyze.IsArchiveDir(archive))
		var opts []Option
		if parallel {
			opts = append(opts, WithParallel())
		}
		deleter := NewDeleter([]string{root}, opts...)

		for entry := range archive.GetFilesLocked(fs.SortByName, fs.SortAsc) {
			assert.ErrorIs(t, deleter.Delete(archive, entry, ModeRemove), ErrInArchive)
			assert.ErrorIs(t, deleter.Delete(archive, entry, ModeEmpty), ErrInArchive)
			assert.ErrorIs(t, deleter.Delete(archive, entry, ModeTrash), ErrInArchive)
		}
		assert.ErrorIs(t, deleter.Delete(tree, archive, ModeEmpty), ErrInArchive)
		assert.FileExists(t, filepath.Join(root, "file"))
		assert.DirExists(t, filepath.Join(root, "dir"))
	}

	// removing the archive itself removes the archive file, nothing else
	tree := scanTree(t, root, true)
	deleter := NewDeleter([]string{root}, WithParallel())
	require.NoError(t, deleter.Delete(tree, child(t, tree, "evil.zip"), ModeRemove))
	assert.NoFileExists(t, filepath.Join(root, "evil.zip"))
	assert.FileExists(t, filepath.Join(root, "file"))
	assert.FileExists(t, filepath.Join(root, "sibling"))
	assert.DirExists(t, filepath.Join(root, "dir"))
}

func TestDeleterEmptyDoesNotWriteThroughSymlink(t *testing.T) {
	root := makeScanRoot(t)
	outside := filepath.Join(t.TempDir(), "precious")
	writeFile(t, outside, "precious")
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	tree := scanTree(t, root, false)
	deleter := NewDeleter([]string{root})

	err := deleter.Delete(tree, child(t, tree, "link"), ModeEmpty)

	assert.ErrorIs(t, err, ErrNotRegular)
	content, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "precious", string(content))

	// removing the link removes the link only
	require.NoError(t, deleter.Delete(tree, child(t, tree, "link"), ModeRemove))
	assert.FileExists(t, outside)
}

// Swapping a directory of the scanned tree for a link after the scan must not
// redirect a removal outside the scanned root, whichever ancestor is swapped.
func TestDeleterRefusesSwappedAncestor(t *testing.T) {
	// outside mirrors the shape of root/dir, so that either swap makes
	// root/dir/b/c resolve to outside/b/c
	for swapped, linkTarget := range map[string]string{
		"dir":                     "",
		filepath.Join("dir", "b"): "b",
	} {
		root := makeScanRoot(t)
		tree := scanTree(t, root, false)
		parent := child(t, child(t, tree, "dir"), "b")
		target := child(t, parent, "c")

		outside := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(outside, "b"), 0o755))
		writeFile(t, filepath.Join(outside, "b", "c"), "precious")
		require.NoError(t, os.RemoveAll(filepath.Join(root, swapped)))
		if err := os.Symlink(filepath.Join(outside, linkTarget), filepath.Join(root, swapped)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		trashCalled := false
		deleter := NewDeleter([]string{root}, WithTrash(func(fs.Item, fs.Item) error {
			trashCalled = true
			return nil
		}))
		for _, mode := range []Mode{ModeRemove, ModeEmpty, ModeTrash} {
			err := deleter.Delete(parent, target, mode)
			assert.ErrorIs(t, err, ErrRelocated, "swapped %s, mode %d", swapped, mode)
		}
		assert.False(t, trashCalled)
		content, err := os.ReadFile(filepath.Join(outside, "b", "c"))
		require.NoError(t, err)
		assert.Equal(t, "precious", string(content))
	}
}

func TestDeleterRemoveIsIdempotentWhenItemIsGone(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)
	require.NoError(t, os.RemoveAll(filepath.Join(root, "dir")))
	deleter := NewDeleter([]string{root})

	require.NoError(t, deleter.Delete(child(t, tree, "dir"), child(t, child(t, tree, "dir"), "a"), ModeRemove))
	require.NoError(t, deleter.Delete(tree, child(t, tree, "dir"), ModeRemove))
	assert.False(t, hasChild(tree, "dir"))
}

func TestDeleterReportsRemovalErrors(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)
	require.NoError(t, os.Remove(filepath.Join(root, "file")))
	deleter := NewDeleter([]string{root})

	err := deleter.Delete(tree, child(t, tree, "file"), ModeEmpty)

	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrRefused))
	assert.True(t, hasChild(tree, "file"))
}

func TestDeleterCheckChangesNothing(t *testing.T) {
	rootA := makeScanRoot(t)
	rootB := makeScanRoot(t)
	treeA := scanTree(t, rootA, false)
	top := analyze.CreateVirtualRootDir(treeA, scanTree(t, rootB, false))
	dir := child(t, treeA, "dir")
	deleter := NewDeleter(analyze.ScannedRootPaths(top))

	for _, mode := range []Mode{ModeRemove, ModeEmpty, ModeTrash} {
		assert.NoError(t, deleter.Check(treeA, dir, mode))
		assert.NoError(t, deleter.Check(treeA, child(t, treeA, "file"), mode))
		assert.ErrorIs(t, deleter.Check(child(t, dir, "b"), dir, mode), ErrNotInDir)
		assert.ErrorIs(t, deleter.Check(nil, dir, mode), ErrNotInDir)
	}
	assert.ErrorIs(t, deleter.Check(top, treeA, ModeRemove), ErrScannedRoot)
	assert.NoError(t, deleter.Check(top, treeA, ModeEmpty))
	assert.ErrorIs(t, NewDeleter(nil).Check(treeA, dir, ModeRemove), ErrOutsideRoots)

	assert.FileExists(t, filepath.Join(rootA, "dir", "a"))
	assert.FileExists(t, filepath.Join(rootA, "file"))
	assert.True(t, hasChild(dir, "a"))
}

func TestDeleterRejectsUnknownMode(t *testing.T) {
	root := makeScanRoot(t)
	tree := scanTree(t, root, false)

	assert.Error(t, NewDeleter([]string{root}).Delete(tree, child(t, tree, "file"), Mode(42)))
	assert.FileExists(t, filepath.Join(root, "file"))
}
