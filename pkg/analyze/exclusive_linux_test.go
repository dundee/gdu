//go:build linux

package analyze

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// exclusiveTestDir creates a scratch directory next to the sources, so that
// the tests run on the filesystem of the checkout rather than on a tmpfs
// /tmp, which has neither FIEMAP nor reflinks.
func exclusiveTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "exclusive_test_")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func writeSynced(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())
}

func allocated(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Sys().(*syscall.Stat_t).Blocks * 512
}

// measure skips the test when the filesystem cannot report extents.
func measure(t *testing.T, path string) int64 {
	t.Helper()
	n, err := exclusiveBytes(path)
	if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOTTY) {
		t.Skipf("FIEMAP is not supported on this filesystem: %v", err)
	}
	require.NoError(t, err)
	return n
}

func TestExclusiveBytesPlainFile(t *testing.T) {
	dir := exclusiveTestDir(t)
	path := filepath.Join(dir, "plain")
	writeSynced(t, path, make([]byte, 1<<20))

	// Nothing is shared, so it matches what the filesystem allocated.
	assert.Equal(t, allocated(t, path), measure(t, path))
}

func TestExclusiveBytesSparseFile(t *testing.T) {
	dir := exclusiveTestDir(t)
	path := filepath.Join(dir, "sparse")

	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(1<<30))
	_, err = f.WriteAt(make([]byte, 4096), 1<<29)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())

	excl := measure(t, path)
	assert.Equal(t, allocated(t, path), excl)
	assert.Greater(t, excl, int64(0))
	assert.Less(t, excl, int64(1<<20), "holes must count as nothing")
}

func TestExclusiveBytesEmptyFile(t *testing.T) {
	dir := exclusiveTestDir(t)
	path := filepath.Join(dir, "empty")
	writeSynced(t, path, nil)

	assert.Equal(t, int64(0), measure(t, path))
}

func TestExclusiveBytesPaging(t *testing.T) {
	dir := exclusiveTestDir(t)
	path := filepath.Join(dir, "fragmented")

	// Write 4 KiB at the start of every other 64 KiB, so extents cannot
	// merge, and read the map back with a tiny page size.
	const extents = 3 * fiemapExtentsPerRequest
	f, err := os.Create(path)
	require.NoError(t, err)
	chunk := make([]byte, 4096)
	for i := range extents {
		_, err = f.WriteAt(chunk, int64(i)*2*65536)
		require.NoError(t, err)
	}
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())

	whole := measure(t, path)
	assert.GreaterOrEqual(t, whole, int64(extents*4096))
	// the file system may also count its own extent tree blocks
	assert.LessOrEqual(t, whole, allocated(t, path))
	assert.Less(t, allocated(t, path)-whole, int64(1<<20))

	for _, perRequest := range []uint32{1, 2, 7, fiemapExtentsPerRequest - 1} {
		got, err := exclusiveBytesPaged(path, perRequest)
		require.NoError(t, err)
		assert.Equal(t, whole, got, "extents per request: %d", perRequest)
	}
}

func TestExclusiveBytesErrors(t *testing.T) {
	dir := exclusiveTestDir(t)

	_, err := exclusiveBytes(filepath.Join(dir, "missing"))
	assert.ErrorIs(t, err, syscall.ENOENT)
}

func TestExclusiveUsageKeepsUsageWhenNotMeasurable(t *testing.T) {
	dir := exclusiveTestDir(t)

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	got, ok := exclusiveUsage(dir, dirInfo, 4096)
	assert.False(t, ok, "directories are not measured")
	assert.Equal(t, int64(4096), got)

	path := filepath.Join(dir, "file")
	writeSynced(t, path, []byte("x"))
	info, err := os.Stat(path)
	require.NoError(t, err)

	got, ok = exclusiveUsage(path, info, 0)
	assert.False(t, ok, "nothing allocated, nothing to measure")
	assert.Equal(t, int64(0), got)

	got, ok = exclusiveUsage(filepath.Join(dir, "gone"), info, 4096)
	assert.False(t, ok, "unreadable file falls back")
	assert.Equal(t, int64(4096), got)

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink("file", link))
	linkInfo, err := os.Lstat(link)
	require.NoError(t, err)
	got, ok = exclusiveUsage(link, linkInfo, 8)
	assert.False(t, ok, "symlinks are not measured")
	assert.Equal(t, int64(8), got)
}

// makeClone creates src with data and dst as a reflink of it. It skips the
// test when the filesystem cannot clone.
func makeClone(t *testing.T, dir string, size int) (src, dst string) {
	t.Helper()
	src = filepath.Join(dir, "src")
	dst = filepath.Join(dir, "dst")
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i%251) + 1
	}
	writeSynced(t, src, data)

	s, err := os.Open(src)
	require.NoError(t, err)
	defer s.Close()
	d, err := os.Create(dst)
	require.NoError(t, err)
	defer d.Close()

	if err = unix.IoctlFileClone(int(d.Fd()), int(s.Fd())); err != nil {
		t.Skipf("FICLONE is not supported on this filesystem: %v", err)
	}
	require.NoError(t, d.Sync())
	return src, dst
}

func TestExclusiveBytesReflink(t *testing.T) {
	dir := exclusiveTestDir(t)
	const size = 1 << 20
	src, dst := makeClone(t, dir, size)

	// Skip, rather than fail, where the filesystem clones but FIEMAP does
	// not flag the shared extents.
	if measure(t, src) != 0 {
		t.Skip("filesystem does not report shared extents")
	}

	assert.Equal(t, int64(0), measure(t, src))
	assert.Equal(t, int64(0), measure(t, dst))

	// Disk usage as st_blocks sees it, and the apparent size, are unchanged
	// by the clone: both files still look full size.
	for _, p := range []string{src, dst} {
		info, err := os.Stat(p)
		require.NoError(t, err)
		assert.Equal(t, int64(size), info.Size())
		assert.GreaterOrEqual(t, allocated(t, p), int64(size))
	}

	// Rewriting part of one copy gives it blocks of its own.
	f, err := os.OpenFile(dst, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteAt([]byte("changed"), 0)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())

	excl := measure(t, dst)
	assert.Greater(t, excl, int64(0))
	assert.Less(t, excl, int64(size), "most of the file is still shared")
}

func TestExtentBytesSplitsSharedFromExclusive(t *testing.T) {
	dir := exclusiveTestDir(t)
	const size = 1 << 19
	src, dst := makeClone(t, dir, size)
	if measure(t, src) != 0 {
		t.Skip("filesystem does not report shared extents")
	}

	excl, shared, err := extentBytes(dst)
	require.NoError(t, err)
	assert.Equal(t, int64(0), excl)
	assert.Equal(t, int64(size), shared)

	f, err := os.OpenFile(dst, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteAt([]byte("changed"), 0)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())

	excl, shared, err = extentBytes(dst)
	require.NoError(t, err)
	assert.Greater(t, excl, int64(0))
	assert.Equal(t, int64(size), excl+shared, "every mapped byte is one or the other")

	for _, perRequest := range []uint32{1, 2} {
		e, s, err := extentBytesPaged(dst, perRequest)
		require.NoError(t, err)
		assert.Equal(t, excl, e)
		assert.Equal(t, shared, s)
	}

	// A plain file shares nothing.
	plain := filepath.Join(dir, "plain")
	writeSynced(t, plain, make([]byte, 1<<16))
	_, shared, err = extentBytes(plain)
	require.NoError(t, err)
	assert.Equal(t, int64(0), shared)
}

func TestAnalyzersRecordSharedUsage(t *testing.T) {
	dir := exclusiveTestDir(t)
	const size = 1 << 20
	makeClone(t, dir, size)
	if measure(t, filepath.Join(dir, "src")) != 0 {
		t.Skip("filesystem does not report shared extents")
	}
	ignoreDir := func(_, _ string) bool { return false }
	ignoreFile := func(_ string) bool { return false }

	scanners := map[string]func(exclusive bool) fs.Item{
		"sequential": func(exclusive bool) fs.Item {
			a := CreateSeqAnalyzer()
			a.SetExclusiveSize(exclusive)
			r := a.AnalyzeDir(dir, ignoreDir, ignoreFile)
			a.GetDone().Wait()
			return r
		},
		"parallel": func(exclusive bool) fs.Item {
			a := CreateAnalyzer()
			a.SetExclusiveSize(exclusive)
			r := a.AnalyzeDir(dir, ignoreDir, ignoreFile)
			a.GetDone().Wait()
			return r
		},
		"stable-order": func(exclusive bool) fs.Item {
			a := CreateStableOrderAnalyzer()
			a.SetExclusiveSize(exclusive)
			r := a.AnalyzeDir(dir, ignoreDir, ignoreFile)
			a.GetDone().Wait()
			return r
		},
	}
	for name, scan := range scanners {
		t.Run(name, func(t *testing.T) {
			for _, exclusive := range []bool{false, true} {
				root := scan(exclusive)
				for f := range root.GetFiles(fs.SortByName, fs.SortAsc) {
					file, ok := f.(*File)
					require.True(t, ok)
					if exclusive {
						assert.Equal(t, int64(size), file.SharedUsage, f.GetName())
					} else {
						assert.Equal(t, int64(0), file.SharedUsage, "only measured with exclusive size")
					}
				}
			}
		})
	}
}

func TestHardLinksCountedOnce(t *testing.T) {
	dir := exclusiveTestDir(t)
	a := filepath.Join(dir, "a")
	writeSynced(t, a, make([]byte, 1<<20))
	measure(t, a)
	require.NoError(t, os.Link(a, filepath.Join(dir, "b")))

	analyzer := CreateSeqAnalyzer()
	analyzer.SetExclusiveSize(true)
	root := analyzer.AnalyzeDir(dir, func(_, _ string) bool { return false }, func(_ string) bool { return false })
	analyzer.GetDone().Wait()
	root.UpdateStats(make(fs.HardLinkedItems))

	// same figure as the plain analyzer, which counts a hard link once
	plain := CreateSeqAnalyzer()
	plainRoot := plain.AnalyzeDir(dir, func(_, _ string) bool { return false }, func(_ string) bool { return false })
	plain.GetDone().Wait()
	plainRoot.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, plainRoot.GetUsage(), root.GetUsage())
	assert.Equal(t, allocated(t, a), root.GetUsage())
}

func TestAnalyzersReportExclusiveSize(t *testing.T) {
	dir := exclusiveTestDir(t)
	const size = 1 << 20
	makeClone(t, dir, size)
	if measure(t, filepath.Join(dir, "src")) != 0 {
		t.Skip("filesystem does not report shared extents")
	}

	ignoreDir := func(_, _ string) bool { return false }
	ignoreFile := func(_ string) bool { return false }

	run := func(name string, exclusive bool, scan func() fs.Item) int64 {
		t.Helper()
		root := scan()
		root.UpdateStats(make(fs.HardLinkedItems))
		var usage int64
		for f := range root.GetFiles(fs.SortByName, fs.SortAsc) {
			usage += f.GetUsage()
		}
		t.Logf("%s exclusive=%v usage=%d", name, exclusive, usage)
		return usage
	}

	scanners := map[string]func(exclusive bool) fs.Item{
		"sequential": func(exclusive bool) fs.Item {
			a := CreateSeqAnalyzer()
			a.SetExclusiveSize(exclusive)
			r := a.AnalyzeDir(dir, ignoreDir, ignoreFile)
			a.GetDone().Wait()
			return r
		},
		"parallel": func(exclusive bool) fs.Item {
			a := CreateAnalyzer()
			a.SetExclusiveSize(exclusive)
			r := a.AnalyzeDir(dir, ignoreDir, ignoreFile)
			a.GetDone().Wait()
			return r
		},
		"top-dir": func(exclusive bool) fs.Item {
			a := CreateTopDirAnalyzer()
			a.SetExclusiveSize(exclusive)
			r := a.AnalyzeDir(dir, ignoreDir, ignoreFile)
			a.GetDone().Wait()
			return r
		},
		"sqlite": func(exclusive bool) fs.Item {
			a, err := CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "test.db"))
			require.NoError(t, err)
			t.Cleanup(func() { _ = a.storage.Close() })
			a.SetExclusiveSize(exclusive)
			r := a.AnalyzeDir(dir, ignoreDir, ignoreFile)
			a.GetDone().Wait()
			return r
		},
		"stable-order": func(exclusive bool) fs.Item {
			a := CreateStableOrderAnalyzer()
			a.SetExclusiveSize(exclusive)
			r := a.AnalyzeDir(dir, ignoreDir, ignoreFile)
			a.GetDone().Wait()
			return r
		},
	}
	for name, scan := range scanners {
		t.Run(name, func(t *testing.T) {
			normal := run(name, false, func() fs.Item { return scan(false) })
			exclusive := run(name, true, func() fs.Item { return scan(true) })
			assert.GreaterOrEqual(t, normal, int64(2*size), "block usage counts shared data per file")
			assert.Equal(t, int64(0), exclusive, "everything is shared")
		})
	}
}
