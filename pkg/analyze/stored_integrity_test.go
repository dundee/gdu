package analyze

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// makeStoredFixture creates <tmp>/scan with files, nested directories, a zip
// and a tar archive, and returns its path.
func makeStoredFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "scan")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub", "deeper"), 0o755))
	writeFixtureFile(t, filepath.Join(root, "file"), 100)
	writeFixtureFile(t, filepath.Join(root, "sub", "x"), 2000)
	writeFixtureFile(t, filepath.Join(root, "sub", "deeper", "y"), 30000)

	zipped, err := os.Create(filepath.Join(root, "archive.zip"))
	require.NoError(t, err)
	zw := zip.NewWriter(zipped)
	for _, name := range []string{"inner/f", "g"} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(bytes.Repeat([]byte("z"), 500))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, zipped.Close())

	tarred, err := os.Create(filepath.Join(root, "archive.tar"))
	require.NoError(t, err)
	tw := tar.NewWriter(tarred)
	content := bytes.Repeat([]byte("t"), 700)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "dir/t1", Mode: 0o600, Size: int64(len(content))}))
	_, err = tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, tarred.Close())
	return root
}

func writeFixtureFile(t *testing.T, path string, size int) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o600))
}

func noIgnoredDirs(string, string) bool { return false }

// scanStored scans root into a new badger database and computes the totals,
// as a session does, and returns the tree and the database path.
func scanStored(t *testing.T, root string, archives bool) (fs.Item, string) {
	t.Helper()
	db := filepath.Join(t.TempDir(), "gdu.badger")
	analyzer := CreateStoredAnalyzer(db)
	tree := analyzer.AnalyzeDir(context.Background(), root, common.ScanOptions{IgnoreDir: noIgnoredDirs, ArchiveBrowsing: archives})
	tree.UpdateStats(false)
	return tree, db
}

func scanInMemory(t *testing.T, root string, archives bool) fs.Item {
	t.Helper()
	analyzer := CreateAnalyzer()
	tree := analyzer.AnalyzeDir(context.Background(), root, common.ScanOptions{IgnoreDir: noIgnoredDirs, ArchiveBrowsing: archives})
	tree.UpdateStats(false)
	return tree
}

type storedStats struct {
	dir                bool
	size, usage, count int64
}

// flatten maps the path of every item of the tree to its stats, reading the
// tree the way callers that do not hold its locks do.
func flatten(item fs.Item) map[string]storedStats {
	out := map[string]storedStats{}
	var walk func(fs.Item)
	walk = func(it fs.Item) {
		out[it.GetPath()] = storedStats{it.IsDir(), it.GetSize(), it.GetUsage(), it.GetItemCount()}
		if it.IsDir() {
			for child := range it.GetFilesLocked(fs.SortByName, fs.SortAsc) {
				walk(child)
			}
		}
	}
	walk(item)
	return out
}

// entry returns the entry of dir named name, picked the way the TUI picks
// items: listing dir under its read lock.
func entry(t *testing.T, dir fs.Item, name string) fs.Item {
	t.Helper()
	unlock := dir.RLock()
	defer unlock()
	for child := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		if child.GetName() == name {
			return child
		}
	}
	t.Fatalf("%s has no entry %q", dir.GetPath(), name)
	return nil
}

func hasEntry(dir fs.Item, name string) bool {
	for child := range dir.GetFilesLocked(fs.SortByName, fs.SortAsc) {
		if child.GetName() == name {
			return true
		}
	}
	return false
}

// Everyone reading the tree without holding its locks, such as the JSON
// export, the web UI and the cleanup classification, must see the entries
// with their totals, not placeholders.
func TestStoredTreeReadsLikeInMemoryTree(t *testing.T) {
	root := makeStoredFixture(t)
	stored, _ := scanStored(t, root, false)

	assert.Equal(t, flatten(scanInMemory(t, root, false)), flatten(stored))
}

func TestStoredTreeKeepsArchives(t *testing.T) {
	root := makeStoredFixture(t)
	stored, db := scanStored(t, root, true)
	want := flatten(scanInMemory(t, root, true))

	assert.Equal(t, want, flatten(stored))

	loaded, err := CreateStoredAnalyzer(db).LoadAnalysis(root)
	require.NoError(t, err)
	assert.Equal(t, want, flatten(loaded))
	assert.True(t, IsArchiveDir(entry(t, loaded, "archive.zip")))
	assert.True(t, IsArchiveDir(entry(t, loaded, "archive.tar")))
	assert.True(t, IsArchiveEntry(entry(t, entry(t, loaded, "archive.zip"), "g")))
}

// An archive entry named "../x" must not overwrite the record of the
// directory holding the archive.
func TestStoredArchiveEntriesCannotOverwriteOtherRecords(t *testing.T) {
	root := makeStoredFixture(t)
	zipped, err := os.Create(filepath.Join(root, "evil.zip"))
	require.NoError(t, err)
	zw := zip.NewWriter(zipped)
	w, err := zw.Create("../victim/f")
	require.NoError(t, err)
	_, err = w.Write([]byte("evil"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, zipped.Close())

	_, db := scanStored(t, root, true)
	loaded, err := CreateStoredAnalyzer(db).LoadAnalysis(root)

	require.NoError(t, err)
	assert.True(t, hasEntry(loaded, "sub"))
	assert.True(t, hasEntry(loaded, "file"))
	assert.False(t, hasEntry(loaded, "victim"))
}

func TestStoredParentsAreTheDirectoriesListingThem(t *testing.T) {
	root := makeStoredFixture(t)
	stored, db := scanStored(t, root, false)
	loaded, err := CreateStoredAnalyzer(db).LoadAnalysis(root)
	require.NoError(t, err)

	for _, tree := range []fs.Item{stored, loaded} {
		assert.Nil(t, tree.GetParent())
		sub := entry(t, tree, "sub")
		assert.Same(t, tree, sub.GetParent())
		assert.Same(t, sub, entry(t, sub, "x").GetParent())
		deeper := entry(t, sub, "deeper")
		assert.Same(t, sub, deeper.GetParent())
		assert.Same(t, deeper, entry(t, deeper, "y").GetParent())
		// listing again yields the same items
		assert.Same(t, sub, entry(t, tree, "sub"))
	}
}

func TestStoredRemoveDirectoryIsStored(t *testing.T) {
	root := makeStoredFixture(t)
	tree, db := scanStored(t, root, false)
	sub := entry(t, tree, "sub")
	size, usage, count := tree.GetSize(), tree.GetUsage(), tree.GetItemCount()

	tree.RemoveFile(sub)

	assert.False(t, hasEntry(tree, "sub"))
	assert.Equal(t, size-sub.GetSize(), tree.GetSize())
	assert.Equal(t, usage-sub.GetUsage(), tree.GetUsage())
	assert.Equal(t, count-sub.GetItemCount(), tree.GetItemCount())

	loaded, err := CreateStoredAnalyzer(db).LoadAnalysis(root)
	require.NoError(t, err)
	assert.False(t, hasEntry(loaded, "sub"))
	assert.Equal(t, tree.GetSize(), loaded.GetSize())
	assert.Equal(t, tree.GetItemCount(), loaded.GetItemCount())
}

// Emptying a file replaces it with a zero-sized one; the directory must then
// still be storable and the change must be stored.
func TestStoredReplaceFileThenRemoveAnother(t *testing.T) {
	root := makeStoredFixture(t)
	tree, db := scanStored(t, root, false)
	sub := entry(t, tree, "sub")
	x := entry(t, sub, "x")

	sub.RemoveFile(x)
	sub.AddFile(&File{Name: "x", Flag: x.GetFlag(), Parent: sub})
	sub.RemoveFile(entry(t, sub, "deeper"))

	loaded, err := CreateStoredAnalyzer(db).LoadAnalysis(root)
	require.NoError(t, err)
	loadedSub := entry(t, loaded, "sub")
	assert.Equal(t, int64(0), entry(t, loadedSub, "x").GetSize())
	assert.False(t, hasEntry(loadedSub, "deeper"))
}

// A database written by an older gdu, which stored its directories in
// another form, must be refused with a hint instead of misread.
func TestStoredDatabaseOfOlderFormatIsRefused(t *testing.T) {
	root := makeStoredFixture(t)
	_, db := scanStored(t, root, false)
	storage := NewStorage(db)
	closeFn := storage.Open()
	require.NoError(t, storage.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(root), []byte("not a record"))
	}))
	closeFn()

	_, err := CreateStoredAnalyzer(db).LoadAnalysis(root)

	assert.ErrorContains(t, err, "scan again")
}

// A directory with very many entries makes a record larger than the largest
// batch badger accepts with the sizes gdu sets; it has to go to the value log.
func TestStoredHugeDirectory(t *testing.T) {
	const entries = 300_000
	storage := NewStorage(filepath.Join(t.TempDir(), "gdu.badger"))
	closeFn := storage.Open()
	defer closeFn()
	record := &storedRecord{Entries: make([]storedEntry, entries)}
	for i := range record.Entries {
		record.Entries[i] = storedEntry{Name: fmt.Sprintf("file-with-a-longish-name-%07d", i), Size: int64(i)}
	}

	require.NoError(t, storage.storeRecord("/huge", record))
	loaded, err := storage.loadRecord("/huge")

	require.NoError(t, err)
	assert.Len(t, loaded.Entries, entries)
	assert.Equal(t, record.Entries[entries-1], loaded.Entries[entries-1])
}
