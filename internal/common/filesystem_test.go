package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func mockFilesystemIDs(t *testing.T, ids map[string]uint64) map[string]int {
	t.Helper()
	calls := make(map[string]int)
	orig := getFilesystemID
	getFilesystemID = func(path string) (uint64, bool) {
		calls[path]++
		id, ok := ids[path]
		return id, ok
	}
	t.Cleanup(func() { getFilesystemID = orig })
	return calls
}

func TestIgnoreOtherFilesystemsLooksUpEachDirOnce(t *testing.T) {
	calls := mockFilesystemIDs(t, map[string]uint64{
		"/data":       1,
		"/data/a":     1,
		"/data/b":     1,
		"/data/a/c":   1,
		"/data/a/d":   1,
		"/data/other": 2,
	})

	ui := &UI{}
	ui.SetIgnoreOtherFilesystems(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.False(t, shouldBeIgnored("a", "/data/a"))
	assert.False(t, shouldBeIgnored("b", "/data/b"))
	assert.True(t, shouldBeIgnored("other", "/data/other"))
	assert.False(t, shouldBeIgnored("c", "/data/a/c"))
	assert.False(t, shouldBeIgnored("d", "/data/a/d"))

	assert.Len(t, calls, 6)
	for path, count := range calls {
		assert.Equal(t, 1, count, path)
	}
}

func TestFilesystemIDCacheReplacesCollidingEntry(t *testing.T) {
	calls := mockFilesystemIDs(t, map[string]uint64{})
	cache := newFilesystemIDCache()

	first := "/data/a"
	second := ""
	for i := 0; second == ""; i++ {
		candidate := fmt.Sprintf("/data/%d", i)
		if fnv1a(candidate)%filesystemIDCacheSize == fnv1a(first)%filesystemIDCacheSize {
			second = candidate
		}
	}

	cache.filesystemID(first)
	cache.filesystemID(second)
	cache.filesystemID(first)

	assert.Equal(t, 2, calls[first])
	assert.Equal(t, 1, calls[second])
}

func TestIgnoreOtherFilesystemsIgnoresDirOnOtherFilesystem(t *testing.T) {
	mockFilesystemIDs(t, map[string]uint64{
		"/data":                    1,
		"/data/sub":                1,
		"/data/.snapshots":         1,
		"/data/.snapshots/2024-01": 2, // e.g. btrfs snapshot, not a mount point
	})

	ui := &UI{}
	ui.SetIgnoreOtherFilesystems(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.False(t, shouldBeIgnored("sub", "/data/sub"))
	assert.False(t, shouldBeIgnored(".snapshots", "/data/.snapshots"))
	assert.True(t, shouldBeIgnored("2024-01", "/data/.snapshots/2024-01"))
}

func TestIgnoreOtherFilesystemsDoesNotIgnoreWhenFilesystemUnknown(t *testing.T) {
	mockFilesystemIDs(t, map[string]uint64{"/data": 1})

	ui := &UI{}
	ui.SetIgnoreOtherFilesystems(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.False(t, shouldBeIgnored("missing", "/data/missing"))
	assert.False(t, shouldBeIgnored("data", "/data"))
}

func TestIgnoreOtherFilesystemsCombinedWithOtherIgnores(t *testing.T) {
	mockFilesystemIDs(t, map[string]uint64{
		"/data":       1,
		"/data/abc":   1,
		"/data/.git":  1,
		"/data/other": 2,
		"/data/xxx":   1,
	})

	ui := &UI{}
	ui.SetIgnoreDirPaths([]string{"/data/abc"})
	ui.SetIgnoreHidden(true)
	ui.SetIgnoreOtherFilesystems(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/data/abc"))
	assert.True(t, shouldBeIgnored(".git", "/data/.git"))
	assert.True(t, shouldBeIgnored("other", "/data/other"))
	assert.False(t, shouldBeIgnored("xxx", "/data/xxx"))
}

func TestWithoutIgnoreOtherFilesystemsOtherFilesystemIsNotIgnored(t *testing.T) {
	mockFilesystemIDs(t, map[string]uint64{"/data": 1, "/data/other": 2})

	ui := &UI{}
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.False(t, shouldBeIgnored("other", "/data/other"))
}
