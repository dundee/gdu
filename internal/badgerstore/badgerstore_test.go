package badgerstore

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutGet(t *testing.T) {
	store := New(t.TempDir())
	defer store.Close()

	require.NoError(t, store.Put([]byte("/a"), []byte("record")))
	value, found, err := store.Get([]byte("/a"))

	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, []byte("record"), value)
	_, found, err = store.Get([]byte("/b"))
	require.NoError(t, err)
	assert.False(t, found)
}

// The database opens on first use and stays open, however many operations
// follow, until Close; what was written is there when it is opened again.
func TestStoreStaysOpenUntilClosed(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)
	assert.False(t, store.IsOpen())

	for i := range 20_000 {
		require.NoError(t, store.Put(fmt.Appendf(nil, "/%d", i), fmt.Appendf(nil, "%d", i)))
	}
	assert.True(t, store.IsOpen())
	require.NoError(t, store.Close())
	assert.False(t, store.IsOpen())

	_, _, err := store.Get([]byte("/0"))
	require.ErrorIs(t, err, ErrClosed)
	require.ErrorIs(t, store.Put([]byte("/0"), nil), ErrClosed)
	require.NoError(t, store.Close(), "closing twice is fine")

	reopened := New(dir)
	defer reopened.Close()
	for _, i := range []int{0, 9_999, 19_999} {
		value, found, err := reopened.Get(fmt.Appendf(nil, "/%d", i))
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, fmt.Appendf(nil, "%d", i), value)
	}
}

// Concurrent first uses open one database: badger locks its directory, so
// opening it twice fails.
func TestConcurrentFirstUsesOpenOneDatabase(t *testing.T) {
	store := New(t.TempDir())
	defer store.Close()

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			runtime.Gosched()
			assert.NoError(t, store.Put(fmt.Appendf(nil, "/%d", i), []byte("x")))
		})
	}
	wg.Wait()

	assert.True(t, store.IsOpen())
}

// A database that cannot be opened, as one another store holds, fails the
// operation with an error; it used to panic.
func TestOpenErrorsAreReturned(t *testing.T) {
	dir := t.TempDir()
	holder := New(dir)
	defer holder.Close()
	require.NoError(t, holder.Put([]byte("/a"), nil))

	_, _, err := New(dir).Get([]byte("/a"))

	assert.ErrorContains(t, err, "opening the badger database")
}

func TestOpenErrorOfAnUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs permissions that hold")
	}
	parent := t.TempDir()
	require.NoError(t, os.Chmod(parent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	err := New(filepath.Join(parent, "db")).Put([]byte("/a"), nil)

	assert.ErrorContains(t, err, "opening the badger database")
}
