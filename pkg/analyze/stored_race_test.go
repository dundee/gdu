package analyze

import (
	"context"
	"runtime"
	"sync"
	"testing"

	"github.com/dundee/gdu/v6/internal/badgerstore"
	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Readers of a stored tree, such as the web UI's request handlers, read it at
// once, holding only the tree's read lock, and load its directories as they
// go. Run with -race.
func TestStoredTreeConcurrentReaders(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := createStoredAnalyzer(t.TempDir())
	scanned := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{})
	scanned.UpdateStats(false)
	want := flatten(scanned)
	dir, err := analyzer.LoadAnalysis("test_dir")
	require.NoError(t, err)

	var wg sync.WaitGroup
	got := make([]map[string]storedStats, 4)
	for i := range got {
		wg.Go(func() {
			readLockedStoredDir(dir.(*StoredDir))
			got[i] = flatten(dir)
		})
	}
	wg.Wait()

	for _, g := range got {
		assert.Equal(t, want, g)
	}
}

// TestStoredDirUsesOneDatabaseConcurrently drives the same workload from a
// fresh analyzer, whose first operations, run concurrently, all open the
// database: they must share one badger instance instead of tripping its
// exclusive directory lock.
func TestStoredDirUsesOneDatabaseConcurrently(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := createStoredAnalyzer(t.TempDir())
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{
		IgnoreFile: func(_ string) bool { return false },
	}).(*StoredDir)

	dir.UpdateStats(false)
	db := analyzer.storage.RecordStore.(*badgerstore.Store)
	require.NoError(t, analyzer.Close())
	// a later run, whose readers all open the database at once
	analyzer = NewStoredAnalyzer(badgerstore.New(db.Path()))
	t.Cleanup(func() { _ = analyzer.Close() })
	loaded, err := analyzer.LoadAnalysis("test_dir")
	require.NoError(t, err)
	dir = loaded.(*StoredDir)

	// as the session does: computing the totals changes the tree, reading
	// it does not, and readers open the storage at once
	var tree sync.RWMutex
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				tree.Lock()
				dir.UpdateStats(false)
				tree.Unlock()
				tree.RLock()
				readLockedStoredDir(dir)
				tree.RUnlock()
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()

	assert.True(t, storageIsOpen(analyzer), "the database stays open until Close")
	assert.Positive(t, dir.GetSize())
	require.NoError(t, analyzer.Close())
	assert.False(t, storageIsOpen(analyzer))
}

// readLockedStoredDir mirrors a UI traversal: every scalar accessor plus the
// file listing, which loads children back out of storage.
func readLockedStoredDir(dir *StoredDir) {
	_ = dir.GetName()
	_ = dir.GetFlag()
	_ = dir.GetSize()
	_ = dir.GetUsage()
	_ = dir.GetMtime()
	_ = dir.GetItemCount()
	for item := range dir.GetFiles(fs.SortBySize, fs.SortDesc) {
		_ = item.GetName()
		_ = item.GetFlag()
		_ = item.GetSize()
		_ = item.GetUsage()
		_ = item.GetMtime()
		_ = item.GetItemCount()
	}
}
