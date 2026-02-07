package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestNewSqliteStorage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, storage)
	defer storage.Close()

	// Test that the database is created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestSqliteStorageClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)

	err = storage.Close()
	assert.NoError(t, err)

	// Closing again should not error
	err = storage.Close()
	assert.NoError(t, err)
}

func TestSqliteStorageHasData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Initially no data
	assert.False(t, storage.HasData())

	// Insert an item
	_, err = storage.InsertItem(nil, "root", true, 100, 100, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	// Now has data
	assert.True(t, storage.HasData())
}

func TestSqliteStorageClearItems(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Insert an item
	_, err = storage.InsertItem(nil, "root", true, 100, 100, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)
	assert.True(t, storage.HasData())

	// Clear items
	err = storage.ClearItems()
	assert.NoError(t, err)
	assert.False(t, storage.HasData())
}

func TestSqliteStorageMetadata(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Set metadata
	err = storage.SetMetadata("key1", "value1")
	assert.NoError(t, err)

	// Get metadata
	value, err := storage.GetMetadata("key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", value)

	// Update metadata
	err = storage.SetMetadata("key1", "value2")
	assert.NoError(t, err)

	value, err = storage.GetMetadata("key1")
	assert.NoError(t, err)
	assert.Equal(t, "value2", value)

	// Get non-existent metadata
	_, err = storage.GetMetadata("nonexistent")
	assert.Error(t, err)
}

func TestSqliteStorageInsertAndGetItem(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	mtime := time.Now().Truncate(time.Second)

	// Insert root directory
	rootID, err := storage.InsertItem(nil, "root", true, 1000, 2000, mtime, 5, 0, ' ')
	assert.NoError(t, err)
	assert.Greater(t, rootID, int64(0))

	// Get root item
	root, err := storage.GetRootItem()
	assert.NoError(t, err)
	assert.Equal(t, "root", root.GetName())
	assert.True(t, root.IsDir())
	assert.Equal(t, int64(1000), root.GetSize())
	assert.Equal(t, int64(2000), root.GetUsage())
	assert.Equal(t, 5, root.GetItemCount())
	assert.Equal(t, ' ', root.GetFlag())
	assert.Equal(t, mtime, root.GetMtime())
}

func TestSqliteStorageInsertAndGetChildren(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	mtime := time.Now().Truncate(time.Second)

	// Insert root
	rootID, err := storage.InsertItem(nil, "root", true, 0, 0, mtime, 1, 0, ' ')
	assert.NoError(t, err)

	// Insert children
	_, err = storage.InsertItem(&rootID, "file1.txt", false, 100, 4096, mtime, 1, 0, ' ')
	assert.NoError(t, err)
	_, err = storage.InsertItem(&rootID, "file2.txt", false, 200, 4096, mtime, 1, 12345, 'H')
	assert.NoError(t, err)
	_, err = storage.InsertItem(&rootID, "subdir", true, 500, 8192, mtime, 3, 0, ' ')
	assert.NoError(t, err)

	// Get children
	children, err := storage.GetChildren(rootID)
	assert.NoError(t, err)
	assert.Len(t, children, 3)

	// Verify children names
	names := make([]string, len(children))
	for i, child := range children {
		names[i] = child.GetName()
	}
	assert.Contains(t, names, "file1.txt")
	assert.Contains(t, names, "file2.txt")
	assert.Contains(t, names, "subdir")
}

func TestSqliteStorageUpdateItem(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Insert item
	id, err := storage.InsertItem(nil, "dir", true, 100, 200, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	// Update item
	err = storage.UpdateItem(id, 500, 1000, 10)
	assert.NoError(t, err)

	// Verify update
	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)
	assert.Equal(t, int64(500), item.GetSize())
	assert.Equal(t, int64(1000), item.GetUsage())
	assert.Equal(t, 10, item.GetItemCount())
}

func TestSqliteStorageBulkInsert(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Begin bulk insert
	err = storage.BeginBulkInsert()
	assert.NoError(t, err)

	// Insert many items
	rootID, err := storage.InsertItem(nil, "root", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		_, err = storage.InsertItem(&rootID, "file", false, 100, 4096, time.Now(), 1, 0, ' ')
		assert.NoError(t, err)
	}

	// Update during bulk mode
	err = storage.UpdateItem(rootID, 10000, 20000, 101)
	assert.NoError(t, err)

	// End bulk insert
	err = storage.EndBulkInsert()
	assert.NoError(t, err)

	// Verify
	children, err := storage.GetChildren(rootID)
	assert.NoError(t, err)
	assert.Len(t, children, 100)
}

func TestSqliteStorageHasInode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// No inode initially
	assert.False(t, storage.HasInode(12345))

	// Insert item with inode
	_, err = storage.InsertItem(nil, "file", false, 100, 4096, time.Now(), 1, 12345, 'H')
	assert.NoError(t, err)

	// Now inode exists
	assert.True(t, storage.HasInode(12345))
	assert.False(t, storage.HasInode(99999))
}

func TestSqliteStorageHasInodeBulkMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	err = storage.BeginBulkInsert()
	assert.NoError(t, err)

	// Insert item with inode in bulk mode
	_, err = storage.InsertItem(nil, "file", false, 100, 4096, time.Now(), 1, 12345, 'H')
	assert.NoError(t, err)

	// Check inode during bulk mode (uses prepared statement)
	assert.True(t, storage.HasInode(12345))
	assert.False(t, storage.HasInode(99999))

	err = storage.EndBulkInsert()
	assert.NoError(t, err)
}

func TestSqliteItemGetPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Set up metadata for path resolution
	err = storage.SetMetadata("top_dir_path", "/home/user/testdir")
	assert.NoError(t, err)

	// Insert root
	rootID, err := storage.InsertItem(nil, "testdir", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	// Insert child
	childID, err := storage.InsertItem(&rootID, "file.txt", false, 100, 4096, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	// Get root item
	root, err := storage.GetItemByID(rootID)
	assert.NoError(t, err)
	assert.Equal(t, "/home/user/testdir", root.GetPath())

	// Get child and set parent
	child, err := storage.GetItemByID(childID)
	assert.NoError(t, err)
	child.SetParent(root)
	assert.Equal(t, "/home/user/testdir/file.txt", child.GetPath())
}

func TestSqliteItemGetType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Directory
	dirID, err := storage.InsertItem(nil, "dir", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)
	dir, _ := storage.GetItemByID(dirID)
	assert.Equal(t, "Directory", dir.GetType())

	// File
	fileID, err := storage.InsertItem(nil, "file", false, 100, 4096, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)
	file, _ := storage.GetItemByID(fileID)
	assert.Equal(t, "File", file.GetType())

	// Other (symlink flag)
	otherID, err := storage.InsertItem(nil, "symlink", false, 100, 4096, time.Now(), 1, 0, '@')
	assert.NoError(t, err)
	other, _ := storage.GetItemByID(otherID)
	assert.Equal(t, "Other", other.GetType())
}

func TestSqliteItemGetParent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Insert root and child
	rootID, err := storage.InsertItem(nil, "root", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)
	childID, err := storage.InsertItem(&rootID, "child", false, 100, 4096, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	// Get child
	child, err := storage.GetItemByID(childID)
	assert.NoError(t, err)

	// Get parent (lazy loaded)
	parent := child.GetParent()
	assert.NotNil(t, parent)
	assert.Equal(t, "root", parent.GetName())

	// Second call should use cached parent
	parent2 := child.GetParent()
	assert.Equal(t, parent, parent2)

	// Root item has no parent
	root, err := storage.GetItemByID(rootID)
	assert.NoError(t, err)
	assert.Nil(t, root.GetParent())
}

func TestSqliteItemGetMultiLinkedInode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Insert item with inode
	id, err := storage.InsertItem(nil, "file", false, 100, 4096, time.Now(), 1, 12345, 'H')
	assert.NoError(t, err)

	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)
	assert.Equal(t, uint64(12345), item.GetMultiLinkedInode())
}

func TestSqliteItemGetFiles(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	// Insert root and children with different usages (SortBySize sorts by usage)
	rootID, err := storage.InsertItem(nil, "root", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)
	_, err = storage.InsertItem(&rootID, "small.txt", false, 100, 1000, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)
	_, err = storage.InsertItem(&rootID, "large.txt", false, 1000, 9000, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)
	_, err = storage.InsertItem(&rootID, "medium.txt", false, 500, 5000, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	root, err := storage.GetItemByID(rootID)
	assert.NoError(t, err)

	// Sort by name ascending (alphabetical)
	files := slices.Collect(root.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Len(t, files, 3)
	assert.Equal(t, "large.txt", files[0].GetName())
	assert.Equal(t, "medium.txt", files[1].GetName())
	assert.Equal(t, "small.txt", files[2].GetName())

	// Sort by size descending (largest usage first)
	files = slices.Collect(root.GetFiles(fs.SortBySize, fs.SortDesc))
	assert.Len(t, files, 3)
	assert.Equal(t, "large.txt", files[0].GetName())
	assert.Equal(t, "medium.txt", files[1].GetName())
	assert.Equal(t, "small.txt", files[2].GetName())
}

func TestSqliteItemGetFilesLocked(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	rootID, err := storage.InsertItem(nil, "root", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)
	_, err = storage.InsertItem(&rootID, "file.txt", false, 100, 4096, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	root, err := storage.GetItemByID(rootID)
	assert.NoError(t, err)

	// GetFilesLocked should work same as GetFiles
	files := slices.Collect(root.GetFilesLocked(fs.SortByName, fs.SortAsc))
	assert.Len(t, files, 1)
	assert.Equal(t, "file.txt", files[0].GetName())
}

func TestSqliteItemRLock(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	id, err := storage.InsertItem(nil, "root", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)

	// RLock should return unlock function
	unlock := item.RLock()
	assert.NotNil(t, unlock)
	unlock()
}

func TestSqliteItemGetItemStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	id, err := storage.InsertItem(nil, "dir", true, 1000, 2000, time.Now(), 5, 0, ' ')
	assert.NoError(t, err)

	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)

	count, size, usage := item.GetItemStats(make(fs.HardLinkedItems))
	assert.Equal(t, 5, count)
	assert.Equal(t, int64(1000), size)
	assert.Equal(t, int64(2000), usage)
}

func TestSqliteItemUpdateStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	id, err := storage.InsertItem(nil, "dir", true, 1000, 2000, time.Now(), 5, 0, ' ')
	assert.NoError(t, err)

	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)

	// UpdateStats is a no-op for SqliteItem
	item.UpdateStats(make(fs.HardLinkedItems))
	// Just verify it doesn't panic
}

func TestSqliteItemAddFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	id, err := storage.InsertItem(nil, "dir", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)

	// AddFile is a no-op for SqliteItem
	item.AddFile(nil)
	// Just verify it doesn't panic
}

func TestSqliteItemRemoveFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	id, err := storage.InsertItem(nil, "dir", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)

	// RemoveFile is a no-op for SqliteItem
	item.RemoveFile(nil)
	// Just verify it doesn't panic
}

func TestSqliteItemRemoveFileByName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	id, err := storage.InsertItem(nil, "dir", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)

	// RemoveFileByName is a no-op for SqliteItem
	item.RemoveFileByName("test")
	// Just verify it doesn't panic
}

func TestSqliteItemEncodeJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := NewSqliteStorage(dbPath)
	assert.NoError(t, err)
	defer storage.Close()

	id, err := storage.InsertItem(nil, "dir", true, 0, 0, time.Now(), 1, 0, ' ')
	assert.NoError(t, err)

	item, err := storage.GetItemByID(id)
	assert.NoError(t, err)

	// EncodeJSON returns nil (simplified implementation)
	err = item.EncodeJSON(nil, false)
	assert.NoError(t, err)
}

func TestCreateSqliteAnalyzer(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, analyzer)
	defer analyzer.storage.Close()
}

func TestSqliteAnalyzerSetFollowSymlinks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	analyzer.SetFollowSymlinks(true)
	assert.True(t, analyzer.followSymlinks)
	analyzer.SetFollowSymlinks(false)
	assert.False(t, analyzer.followSymlinks)
}

func TestSqliteAnalyzerSetShowAnnexedSize(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	analyzer.SetShowAnnexedSize(true)
	assert.True(t, analyzer.gitAnnexedSize)
	analyzer.SetShowAnnexedSize(false)
	assert.False(t, analyzer.gitAnnexedSize)
}

func TestSqliteAnalyzerSetArchiveBrowsing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	analyzer.SetArchiveBrowsing(true)
	assert.True(t, analyzer.archiveBrowsing)
	analyzer.SetArchiveBrowsing(false)
	assert.False(t, analyzer.archiveBrowsing)
}

func TestSqliteAnalyzerSetTimeFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	filter := func(mtime time.Time) bool { return true }
	analyzer.SetTimeFilter(filter)
	assert.NotNil(t, analyzer.matchesTimeFilterFn)
}

func TestSqliteAnalyzerSetFileTypeFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	filter := func(name string) bool { return false }
	analyzer.SetFileTypeFilter(filter)
	assert.NotNil(t, analyzer.ignoreFileType)
}

func TestSqliteAnalyzerGetProgressChan(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	progressChan := analyzer.GetProgressChan()
	assert.NotNil(t, progressChan)
}

func TestSqliteAnalyzerGetDone(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	doneChan := analyzer.GetDone()
	assert.NotNil(t, doneChan)
}

func TestSqliteAnalyzerResetProgress(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	analyzer.ResetProgress()
	assert.NotNil(t, analyzer.progress)
	assert.NotNil(t, analyzer.progressChan)
	assert.NotNil(t, analyzer.progressOutChan)
	assert.NotNil(t, analyzer.doneChan)
}

func TestSqliteAnalyzerAnalyzeDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SqliteItem)

	analyzer.GetDone().Wait()

	// Test dir info
	assert.Equal(t, "test_dir", dir.GetName())
	assert.True(t, dir.IsDir())
	assert.Equal(t, 5, dir.GetItemCount())
	// Size should include directory overhead + file sizes: 4096*3 + 7 bytes
	assert.Equal(t, int64(7+4096*3), dir.GetSize())

	// Test dir tree
	files := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, 1, len(files))
	assert.Equal(t, "nested", files[0].GetName())

	nested := files[0].(*SqliteItem)
	nestedFiles := slices.Collect(nested.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, 2, len(nestedFiles))
	assert.Equal(t, "file2", nestedFiles[0].GetName())
	assert.Equal(t, "subnested", nestedFiles[1].GetName())

	// Test file
	assert.Equal(t, int64(2), nestedFiles[0].GetSize())

	subnested := nestedFiles[1].(*SqliteItem)
	subnestedFiles := slices.Collect(subnested.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, "file", subnestedFiles[0].GetName())
	assert.Equal(t, int64(5), subnestedFiles[0].GetSize())
}

func TestSqliteAnalyzerIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return true }, func(_ string) bool { return false },
	).(*SqliteItem)

	analyzer.GetDone().Wait()

	assert.Equal(t, "test_dir", dir.GetName())
	assert.Equal(t, 1, dir.GetItemCount())
}

func TestSqliteAnalyzerIgnoreFileType(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	// Ignore all files
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return true },
	).(*SqliteItem)

	analyzer.GetDone().Wait()

	// Only directories should remain
	assert.Equal(t, "test_dir", dir.GetName())
	assert.Equal(t, 3, dir.GetItemCount()) // test_dir, nested, subnested
}

func TestSqliteAnalyzerHardlinks(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// Create hard link
	err := os.Link("test_dir/nested/file2", "test_dir/nested/file3")
	assert.NoError(t, err)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SqliteItem)

	analyzer.GetDone().Wait()

	// file2 and file3 are counted just once for size but twice for item count
	assert.Equal(t, int64(7+4096*3), dir.GetSize())
	assert.Equal(t, 6, dir.GetItemCount())

	// Check hard link flag
	nested := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))[0].(*SqliteItem)
	nestedFiles := slices.Collect(nested.GetFiles(fs.SortByName, fs.SortAsc))

	var file3 *SqliteItem
	for _, f := range nestedFiles {
		if f.GetName() == "file3" {
			file3 = f.(*SqliteItem)
			break
		}
	}
	assert.NotNil(t, file3)
	assert.Equal(t, 'H', file3.GetFlag())
}

func TestSqliteAnalyzerSymlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// Create symlink
	err := os.Symlink("test_dir/nested/file2", "test_dir/nested/file3")
	assert.NoError(t, err)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SqliteItem)

	analyzer.GetDone().Wait()

	// Check symlink flag
	nested := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))[0].(*SqliteItem)
	nestedFiles := slices.Collect(nested.GetFiles(fs.SortByName, fs.SortAsc))

	var file3 *SqliteItem
	for _, f := range nestedFiles {
		if f.GetName() == "file3" {
			file3 = f.(*SqliteItem)
			break
		}
	}
	assert.NotNil(t, file3)
	assert.Equal(t, '@', file3.GetFlag())
}

func TestSqliteAnalyzerFollowSymlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// Create symlink to file2
	err := os.Symlink("./file2", "test_dir/nested/file3")
	assert.NoError(t, err)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	analyzer.SetFollowSymlinks(true)

	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SqliteItem)

	analyzer.GetDone().Wait()

	// With followSymlinks, file3 should have same size as file2
	nested := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))[0].(*SqliteItem)
	nestedFiles := slices.Collect(nested.GetFiles(fs.SortByName, fs.SortAsc))

	var file3 *SqliteItem
	for _, f := range nestedFiles {
		if f.GetName() == "file3" {
			file3 = f.(*SqliteItem)
			break
		}
	}
	assert.NotNil(t, file3)
	assert.Equal(t, int64(2), file3.GetSize())
	assert.Equal(t, ' ', file3.GetFlag()) // Not a symlink flag when followed
}

func TestSqliteAnalyzerTimeFilter(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	// Filter out all files (mtime filter that always returns false)
	analyzer.SetTimeFilter(func(mtime time.Time) bool { return false })

	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SqliteItem)

	analyzer.GetDone().Wait()

	// Only directories should remain
	assert.Equal(t, 3, dir.GetItemCount()) // test_dir, nested, subnested
}

func TestSqliteAnalyzerLoadFromExisting(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	// First analysis
	analyzer1, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)

	dir1 := analyzer1.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SqliteItem)
	analyzer1.GetDone().Wait()

	assert.Equal(t, "test_dir", dir1.GetName())
	assert.Equal(t, 5, dir1.GetItemCount())

	analyzer1.storage.Close()

	// Second analysis should load from existing data
	analyzer2, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer2.storage.Close()

	dir2 := analyzer2.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SqliteItem)
	analyzer2.GetDone().Wait()

	assert.Equal(t, "test_dir", dir2.GetName())
	assert.Equal(t, 5, dir2.GetItemCount())
}

func TestSqliteAnalyzerProgress(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	analyzer, err := CreateSqliteAnalyzer(dbPath)
	assert.NoError(t, err)
	defer analyzer.storage.Close()

	// Start analysis in goroutine
	go func() {
		analyzer.AnalyzeDir(
			"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
		)
	}()

	// Receive at least one progress update
	select {
	case progress := <-analyzer.GetProgressChan():
		assert.GreaterOrEqual(t, progress.TotalSize, int64(0))
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for progress")
	}

	analyzer.GetDone().Wait()
}

func BenchmarkSqliteAnalyzeDir(b *testing.B) {
	fin := testdir.CreateTestDir()
	defer fin()

	for i := 0; i < b.N; i++ {
		dbPath := filepath.Join(b.TempDir(), "test.db")
		analyzer, _ := CreateSqliteAnalyzer(dbPath)
		analyzer.AnalyzeDir(
			"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
		)
		analyzer.GetDone().Wait()
		analyzer.storage.Close()
	}
}
