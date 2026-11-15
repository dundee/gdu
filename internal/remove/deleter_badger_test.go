package remove

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/analyze/badger"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// scanBadger scans root into a badger database and computes the totals, as
// a session does, and returns the tree and the database path.
func scanBadger(t *testing.T, root string, archives bool) (fs.Item, string) {
	t.Helper()
	db := filepath.Join(t.TempDir(), "gdu.badger")
	analyzer := badger.CreateAnalyzer(db)
	tree := analyzer.AnalyzeDir(context.Background(), root, analyze.Options{ArchiveBrowsing: archives})
	tree.UpdateStats(false)
	return tree, db
}

func loadBadger(t *testing.T, db, root string) fs.Item {
	t.Helper()
	tree, err := badger.CreateAnalyzer(db).LoadAnalysis(root)
	require.NoError(t, err)
	return tree
}

// listed returns the entry of dir named name, picked the way the TUI picks
// items: listing dir under its read lock.
func listed(t *testing.T, dir fs.Item, name string) fs.Item {
	t.Helper()
	for entry := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		if entry.GetName() == name {
			return entry
		}
	}
	t.Fatalf("%s has no entry %q", dir.GetPath(), name)
	return nil
}

// Emptying a file and then changing the same directory used to overflow the
// stack while storing the directory.
func TestDeleterEmptiesAndRemovesWithBadger(t *testing.T) {
	root := makeScanRoot(t)
	tree, db := scanBadger(t, root, false)
	deleter := NewDeleter([]string{root})
	dir := listed(t, tree, "dir")

	require.NoError(t, deleter.Delete(dir, listed(t, dir, "a"), ModeEmpty))
	require.NoError(t, deleter.Delete(dir, listed(t, dir, "b"), ModeRemove))

	info, err := os.Stat(filepath.Join(root, "dir", "a"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
	assert.NoDirExists(t, filepath.Join(root, "dir", "b"))

	loaded := loadBadger(t, db, root)
	loadedDir := listed(t, loaded, "dir")
	assert.Equal(t, int64(0), listed(t, loadedDir, "a").GetSize())
	assert.False(t, hasChild(loadedDir, "b"))
	assert.Equal(t, tree.GetSize(), loaded.GetSize())
}

func TestDeleterRemovesDirectoryWithBadger(t *testing.T) {
	root := makeScanRoot(t)
	tree, db := scanBadger(t, root, false)
	dir := listed(t, tree, "dir")
	size, count := tree.GetSize(), tree.GetItemCount()

	require.NoError(t, NewDeleter([]string{root}).Delete(tree, dir, ModeRemove))

	assert.False(t, hasChild(tree, "dir"))
	assert.Equal(t, size-dir.GetSize(), tree.GetSize())
	assert.Equal(t, count-dir.GetItemCount(), tree.GetItemCount())

	loaded := loadBadger(t, db, root)
	assert.False(t, hasChild(loaded, "dir"))
	assert.Equal(t, tree.GetSize(), loaded.GetSize())
}

func TestDeleterNeverTouchesStoredArchiveEntries(t *testing.T) {
	root := makeScanRoot(t)
	writeZip(t, filepath.Join(root, "archive.zip"), "inner/f", "g")
	tree, _ := scanBadger(t, root, true)
	archive := listed(t, tree, "archive.zip")
	deleter := NewDeleter([]string{root})

	assert.ErrorIs(t, deleter.Delete(archive, listed(t, archive, "g"), ModeRemove), ErrInArchive)
	assert.ErrorIs(t, deleter.Delete(archive, listed(t, archive, "inner"), ModeRemove), ErrInArchive)
	assert.ErrorIs(t, deleter.Delete(tree, archive, ModeEmpty), ErrInArchive)

	require.NoError(t, deleter.Delete(tree, archive, ModeRemove))
	assert.NoFileExists(t, filepath.Join(root, "archive.zip"))
	assert.FileExists(t, filepath.Join(root, "file"))
}
