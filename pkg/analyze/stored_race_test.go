package analyze

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

// TestStoredDirUpdateStatsLockedAccessors exercises the same reader/writer mix
// on a StoredDir that the web UI and TUI produce: stats updates overlapping
// readers that go through the synchronized accessors. Run with -race to catch
// any field written outside Dir.m.
func TestStoredDirUpdateStatsLockedAccessors(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer(t.TempDir())
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*StoredDir)
	analyzer.GetDone().Wait()

	// Hold the database open for the whole test so every worker shares one
	// badger instance instead of opening and closing it per call.
	closeFn := DefaultStorage.Open()
	defer closeFn()

	// updateStats is idempotent over a static tree, so the concurrent run must
	// converge on the values a single-threaded run produces.
	dir.UpdateStats(make(fs.HardLinkedItems))
	wantCount, wantSize, wantUsage := dir.GetItemCount(), dir.GetSize(), dir.GetUsage()

	const workers = 4
	const rounds = 25

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				dir.UpdateStats(make(fs.HardLinkedItems))
				runtime.Gosched()
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				readLockedStoredDir(dir)
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, wantCount, dir.GetItemCount())
	assert.Equal(t, wantSize, dir.GetSize())
	assert.Equal(t, wantUsage, dir.GetUsage())
}

// TestStoredDirOpensStorageOnDemandConcurrently drives the same workload with
// no pinned database handle, so every operation has to open storage itself.
// Reference counting must let those overlapping opens share one badger
// instance instead of tripping its exclusive directory lock.
func TestStoredDirOpensStorageOnDemandConcurrently(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer(t.TempDir())
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*StoredDir)
	analyzer.GetDone().Wait()

	assert.False(t, DefaultStorage.IsOpen(), "the analyzer should have closed storage on the way out")

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				dir.UpdateStats(make(fs.HardLinkedItems))
				readLockedStoredDir(dir)
				runtime.Gosched()
			}
		}()
	}
	wg.Wait()

	assert.False(t, DefaultStorage.IsOpen(), "every reference must be released again")
	assert.Positive(t, dir.GetSize())
}

// TestStoredDirGetFilesUnderCallerHeldReadLock pins the caller-locked half of
// the fs.Item contract: tui.(*UI).showDir holds RLock across the whole
// GetFiles iteration, so loading children lazily must not re-enter Dir.m.
// sync.RWMutex is not reentrant, so a write lock there self-deadlocks, and
// even a read lock deadlocks once a writer has queued behind the caller.
func TestStoredDirGetFilesUnderCallerHeldReadLock(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer(t.TempDir())
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*StoredDir)
	analyzer.GetDone().Wait()

	dir.UpdateStats(make(fs.HardLinkedItems))

	done := make(chan int, 1)
	go func() {
		unlock := dir.RLock()
		defer unlock()

		// Queue a writer on the very mutex the caller holds for reading, so
		// the read-lock variant of the bug is covered too.
		queued := make(chan struct{})
		go func() {
			close(queued)
			dir.SetFlag('!')
		}()
		<-queued
		time.Sleep(50 * time.Millisecond)

		var count int
		for range dir.GetFiles(fs.SortByName, fs.SortAsc) {
			count++
		}
		done <- count
	}()

	select {
	case count := <-done:
		assert.Positive(t, count)
	case <-time.After(30 * time.Second):
		t.Fatal("GetFiles deadlocked while the caller held the directory read lock")
	}
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
	storage := NewStorage(t.TempDir(), "/some/path")

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
	storage := NewStorage(t.TempDir(), "/some/path")

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
