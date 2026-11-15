package analyze

import (
	"context"
	"runtime"
	"sync"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Readers of a stored tree, such as the web UI's request handlers, read it at
// once, holding only the tree's read lock, and load its directories as they
// go. Run with -race.
func TestStoredTreeConcurrentReaders(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer(t.TempDir())
	scanned := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{})
	scanned.UpdateStats(false)
	want := flatten(scanned)
	dir, err := analyzer.LoadAnalysis("test_dir")
	require.NoError(t, err)

	closeFn := analyzer.storage.Open()
	defer closeFn()

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

// TestStoredDirOpensStorageOnDemandConcurrently drives the same workload with
// no pinned database handle, so every operation has to open storage itself.
// Reference counting must let those overlapping opens share one badger
// instance instead of tripping its exclusive directory lock.
func TestStoredDirOpensStorageOnDemandConcurrently(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer(t.TempDir())
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{
		IgnoreFile: func(_ string) bool { return false },
	}).(*StoredDir)

	assert.False(t, analyzer.storage.IsOpen(), "the analyzer should have closed storage on the way out")

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

	assert.False(t, analyzer.storage.IsOpen(), "every reference must be released again")
	assert.Positive(t, dir.GetSize())
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

// TestStorageOpenIsReferenceCounted covers the storage contract StoredDir
// relies on: badger locks its directory, so overlapping Open calls must share
// one instance and only the last closer may actually close it.
func TestStorageOpenIsReferenceCounted(t *testing.T) {
	storage := NewStorage(t.TempDir())

	outer := storage.Open()
	assert.True(t, storage.IsOpen())

	inner := storage.Open()
	inner()
	assert.True(t, storage.IsOpen(), "inner closer must not close the shared database")

	outer()
	assert.False(t, storage.IsOpen())

	// closers are idempotent
	outer()
	inner()
	assert.False(t, storage.IsOpen())
}

// TestStorageOpenConcurrent asserts concurrent Open/close pairs never trip
// badger's exclusive directory lock, which would panic inside Open.
func TestStorageOpenConcurrent(t *testing.T) {
	storage := NewStorage(t.TempDir())

	pin := storage.Open()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				closeFn := storage.Open()
				assert.True(t, storage.IsOpen())
				closeFn()
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()

	assert.True(t, storage.IsOpen())
	pin()
	assert.False(t, storage.IsOpen())
}
