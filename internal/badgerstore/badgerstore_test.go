package badgerstore

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutGet(t *testing.T) {
	store := New(t.TempDir())
	closeFn := store.Open()
	defer closeFn()

	require.NoError(t, store.Put([]byte("/a"), []byte("record")))
	value, err := store.Get([]byte("/a"))

	require.NoError(t, err)
	assert.Equal(t, []byte("record"), value)
	_, err = store.Get([]byte("/b"))
	assert.ErrorContains(t, err, "/b")
}

// The database is reopened every reopenEvery operations, also while a caller
// holds it open, and nothing written before is lost.
func TestReopenKeepsRecords(t *testing.T) {
	store := New(t.TempDir())
	closeFn := store.Open()
	defer closeFn()

	for i := range reopenEvery + 10 {
		require.NoError(t, store.Put(fmt.Appendf(nil, "/%d", i), fmt.Appendf(nil, "%d", i)))
	}
	for _, i := range []int{0, reopenEvery - 1, reopenEvery + 9} {
		value, err := store.Get(fmt.Appendf(nil, "/%d", i))
		require.NoError(t, err)
		assert.Equal(t, fmt.Appendf(nil, "%d", i), value)
	}
	assert.True(t, store.IsOpen())
}

// TestOpenIsReferenceCounted covers the contract analyze.RecordStore
// asks for: badger locks its directory, so overlapping Open calls must share
// one instance and only the last closer may actually close it.
func TestOpenIsReferenceCounted(t *testing.T) {
	storage := New(t.TempDir())

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

// TestOpenConcurrent asserts concurrent Open/close pairs never trip
// badger's exclusive directory lock, which would panic inside Open.
func TestOpenConcurrent(t *testing.T) {
	storage := New(t.TempDir())

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
