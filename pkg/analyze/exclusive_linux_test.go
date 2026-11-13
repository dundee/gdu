//go:build linux

package analyze

import (
	"context"
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
// /tmp, which has neither FIEMAP nor reflinks. A read-only checkout falls back
// to the temporary directory.
func exclusiveTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "exclusive_test_")
	if err != nil {
		return t.TempDir()
	}
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
	n, _, err := extentBytes(path, fiemapExtentsPerRequest)
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
		got, _, err := extentBytes(path, perRequest)
		require.NoError(t, err)
		assert.Equal(t, whole, got, "extents per request: %d", perRequest)
	}
}

func TestExclusiveBytesErrors(t *testing.T) {
	dir := exclusiveTestDir(t)

	_, _, err := extentBytes(filepath.Join(dir, "missing"), fiemapExtentsPerRequest)
	assert.ErrorIs(t, err, syscall.ENOENT)
}

func TestExclusiveUsageKeepsUsageWhenNotMeasurable(t *testing.T) {
	dir := exclusiveTestDir(t)

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	got, shared := new(extentMeter).measure(dir, dirInfo, 4096)
	assert.Equal(t, int64(4096), got, "directories are not measured")
	assert.Equal(t, int64(0), shared)

	path := filepath.Join(dir, "file")
	writeSynced(t, path, []byte("x"))
	info, err := os.Stat(path)
	require.NoError(t, err)

	got, _ = new(extentMeter).measure(path, info, 0)
	assert.Equal(t, int64(0), got, "nothing allocated, nothing to measure")

	got, shared = new(extentMeter).measure(filepath.Join(dir, "gone"), info, 4096)
	assert.Equal(t, int64(4096), got, "unreadable file falls back")
	assert.Equal(t, int64(0), shared)

	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink("file", link))
	linkInfo, err := os.Lstat(link)
	require.NoError(t, err)
	got, _ = new(extentMeter).measure(link, linkInfo, 8)
	assert.Equal(t, int64(8), got, "symlinks are not measured")
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

// fiemapFlagSync is FIEMAP_FLAG_SYNC: write the file back before mapping it.
const fiemapFlagSync = 0x00000001

// requireSharedExtents skips the test unless FIEMAP flags the extents of the
// cloned file as shared, which is not the case on every filesystem that
// accepts FICLONE.
func requireSharedExtents(t *testing.T, path string) {
	t.Helper()
	_, shared, err := extentBytes(path, fiemapExtentsPerRequest)
	if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOTTY) {
		t.Skipf("FIEMAP is not supported on this filesystem: %v", err)
	}
	require.NoError(t, err)
	if shared == 0 {
		t.Skip("filesystem does not report shared extents")
	}
}

// syncFiemap makes every FIEMAP call write back the file first. fsync on the
// writing descriptor is not enough on an overlayfs mounted with fsync=volatile,
// as container engines do by default: the overlay then skips the flush, the
// copy-on-write data stays delayed on the underlying inode and FIEMAP keeps
// reporting the old shared extent.
func syncFiemap(t *testing.T) {
	t.Helper()
	orig := fiemapIoctl
	fiemapIoctl = func(fd int, req *fiemapRequest) syscall.Errno {
		req.flags |= fiemapFlagSync
		return orig(fd, req)
	}
	t.Cleanup(func() { fiemapIoctl = orig })
}

func TestExclusiveBytesReflink(t *testing.T) {
	dir := exclusiveTestDir(t)
	const size = 1 << 20
	src, dst := makeClone(t, dir, size)
	syncFiemap(t)
	requireSharedExtents(t, src)

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
	syncFiemap(t)
	requireSharedExtents(t, src)

	excl, shared, err := extentBytes(dst, fiemapExtentsPerRequest)
	require.NoError(t, err)
	assert.Equal(t, int64(0), excl)
	assert.Equal(t, int64(size), shared)

	f, err := os.OpenFile(dst, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteAt([]byte("changed"), 0)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())

	excl, shared, err = extentBytes(dst, fiemapExtentsPerRequest)
	require.NoError(t, err)
	assert.Greater(t, excl, int64(0))
	assert.Equal(t, int64(size), excl+shared, "every mapped byte is one or the other")

	for _, perRequest := range []uint32{1, 2} {
		e, s, err := extentBytes(dst, perRequest)
		require.NoError(t, err)
		assert.Equal(t, excl, e)
		assert.Equal(t, shared, s)
	}

	// A plain file shares nothing.
	plain := filepath.Join(dir, "plain")
	writeSynced(t, plain, make([]byte, 1<<16))
	_, shared, err = extentBytes(plain, fiemapExtentsPerRequest)
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
			r := a.AnalyzeDir(context.Background(), dir, Options{
				IgnoreDir:     ignoreDir,
				IgnoreFile:    ignoreFile,
				ExclusiveSize: exclusive,
			})
			return r
		},
		"parallel": func(exclusive bool) fs.Item {
			a := CreateAnalyzer()
			r := a.AnalyzeDir(context.Background(), dir, Options{
				IgnoreDir:     ignoreDir,
				IgnoreFile:    ignoreFile,
				ExclusiveSize: exclusive,
			})
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
	root := analyzer.AnalyzeDir(context.Background(), dir, Options{
		IgnoreFile:    func(_ string) bool { return false },
		ExclusiveSize: true,
	})
	root.UpdateStats(false)

	// same figure as the plain analyzer, which counts a hard link once
	plain := CreateSeqAnalyzer()
	plainRoot := plain.AnalyzeDir(context.Background(), dir, Options{IgnoreFile: func(_ string) bool { return false }})
	plainRoot.UpdateStats(false)

	assert.Equal(t, plainRoot.GetUsage(), root.GetUsage())
	assert.Equal(t, allocated(t, a)+dirOwnUsage(dir), root.GetUsage())
}

func TestAnalyzersMeasureHardLinkedFileOnce(t *testing.T) {
	ignoreDir := func(_, _ string) bool { return false }
	ignoreFile := func(_ string) bool { return false }

	scanners := map[string]func(dir string) fs.Item{
		"top-dir": func(dir string) fs.Item {
			a := CreateTopDirAnalyzer()
			r := a.AnalyzeDir(context.Background(), dir, Options{
				IgnoreDir:     ignoreDir,
				IgnoreFile:    ignoreFile,
				ExclusiveSize: true,
			})
			return r
		},
		"sqlite": func(dir string) fs.Item {
			a, err := CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "test.db"))
			require.NoError(t, err)
			t.Cleanup(func() { _ = a.storage.Close() })
			r := a.AnalyzeDir(context.Background(), dir, Options{
				IgnoreDir:     ignoreDir,
				IgnoreFile:    ignoreFile,
				ExclusiveSize: true,
			})
			return r
		},
	}
	for name, scan := range scanners {
		t.Run(name, func(t *testing.T) {
			dir := exclusiveTestDir(t)
			// below the top level, where the top-dir analyzer skips links
			sub := filepath.Join(dir, "sub")
			require.NoError(t, os.Mkdir(sub, 0o755))
			writeSynced(t, filepath.Join(sub, "a"), make([]byte, 8192))
			require.NoError(t, os.Link(filepath.Join(sub, "a"), filepath.Join(sub, "b")))
			calls := fakeFiemap(t, mapExtents(fiemapExtent{length: 4096, flags: fiemapExtentLast}))

			root := scan(dir)
			root.UpdateStats(false)

			assert.Equal(t, 1, *calls, "the second link is not measured")
			assert.Equal(t, int64(4096), root.GetUsage()-dirOwnUsage(dir)-dirOwnUsage(sub))
		})
	}
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
		root.UpdateStats(false)
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
			r := a.AnalyzeDir(context.Background(), dir, Options{
				IgnoreDir:     ignoreDir,
				IgnoreFile:    ignoreFile,
				ExclusiveSize: exclusive,
			})
			return r
		},
		"parallel": func(exclusive bool) fs.Item {
			a := CreateAnalyzer()
			r := a.AnalyzeDir(context.Background(), dir, Options{
				IgnoreDir:     ignoreDir,
				IgnoreFile:    ignoreFile,
				ExclusiveSize: exclusive,
			})
			return r
		},
		"top-dir": func(exclusive bool) fs.Item {
			a := CreateTopDirAnalyzer()
			r := a.AnalyzeDir(context.Background(), dir, Options{
				IgnoreDir:     ignoreDir,
				IgnoreFile:    ignoreFile,
				ExclusiveSize: exclusive,
			})
			return r
		},
		"sqlite": func(exclusive bool) fs.Item {
			a, err := CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "test.db"))
			require.NoError(t, err)
			t.Cleanup(func() { _ = a.storage.Close() })
			r := a.AnalyzeDir(context.Background(), dir, Options{
				IgnoreDir:     ignoreDir,
				IgnoreFile:    ignoreFile,
				ExclusiveSize: exclusive,
			})
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

// fakeFiemap replaces the FIEMAP ioctl with the given replies, one per call,
// and returns a pointer to the number of calls made.
func fakeFiemap(t *testing.T, replies ...func(req *fiemapRequest) syscall.Errno) *int {
	t.Helper()
	orig := fiemapIoctl
	t.Cleanup(func() { fiemapIoctl = orig })
	calls := 0
	fiemapIoctl = func(_ int, req *fiemapRequest) syscall.Errno {
		reply := replies[min(calls, len(replies)-1)]
		calls++
		return reply(req)
	}
	return &calls
}

func mapExtents(extents ...fiemapExtent) func(req *fiemapRequest) syscall.Errno {
	return func(req *fiemapRequest) syscall.Errno {
		req.mappedCount = uint32(copy(req.extents[:], extents))
		return 0
	}
}

func TestExtentBytesRetriesInterruptedIoctl(t *testing.T) {
	path := filepath.Join(exclusiveTestDir(t), "f")
	writeSynced(t, path, []byte("x"))
	calls := fakeFiemap(t,
		func(*fiemapRequest) syscall.Errno { return syscall.EINTR },
		mapExtents(fiemapExtent{length: 4096, flags: fiemapExtentLast}),
	)

	excl, shared, err := extentBytes(path, fiemapExtentsPerRequest)
	require.NoError(t, err)
	assert.Equal(t, int64(4096), excl)
	assert.Equal(t, int64(0), shared)
	assert.Equal(t, 2, *calls)
}

func TestExtentBytesReturnsIoctlError(t *testing.T) {
	path := filepath.Join(exclusiveTestDir(t), "f")
	writeSynced(t, path, []byte("x"))
	fakeFiemap(t, func(*fiemapRequest) syscall.Errno { return syscall.EIO })

	_, _, err := extentBytes(path, fiemapExtentsPerRequest)
	assert.ErrorIs(t, err, syscall.EIO)
}

func TestExtentBytesRejectsOverfullReply(t *testing.T) {
	path := filepath.Join(exclusiveTestDir(t), "f")
	writeSynced(t, path, []byte("x"))
	fakeFiemap(t, func(req *fiemapRequest) syscall.Errno {
		req.mappedCount = req.extentCount + 1
		return 0
	})

	_, _, err := extentBytes(path, fiemapExtentsPerRequest)
	assert.ErrorIs(t, err, errFiemapNotSupported)
}

func TestExtentBytesStopsWithoutForwardProgress(t *testing.T) {
	path := filepath.Join(exclusiveTestDir(t), "f")
	writeSynced(t, path, []byte("x"))

	// the extent ends where the request started, and the kernel never says LAST
	fakeFiemap(t, mapExtents(fiemapExtent{logical: 0, length: 0}))
	_, _, err := extentBytes(path, fiemapExtentsPerRequest)
	assert.ErrorIs(t, err, errFiemapNotSupported)

	// logical + length wraps around
	fakeFiemap(t, mapExtents(fiemapExtent{logical: ^uint64(0) - 1, length: 8}))
	_, _, err = extentBytes(path, fiemapExtentsPerRequest)
	assert.ErrorIs(t, err, errFiemapNotSupported)
}

func TestExtentBytesPagesAndSplitsSharedWithFakeKernel(t *testing.T) {
	path := filepath.Join(exclusiveTestDir(t), "f")
	writeSynced(t, path, []byte("x"))
	calls := fakeFiemap(t,
		mapExtents(fiemapExtent{logical: 0, length: 4096}),
		mapExtents(
			fiemapExtent{logical: 4096, length: 8192, flags: fiemapExtentShared},
			fiemapExtent{logical: 12288, length: 4096, flags: fiemapExtentLast},
		),
	)

	excl, shared, err := extentBytes(path, fiemapExtentsPerRequest)
	require.NoError(t, err)
	assert.Equal(t, int64(8192), excl)
	assert.Equal(t, int64(8192), shared)
	assert.Equal(t, 2, *calls)

	fakeFiemap(t, mapExtents())
	excl, shared, err = extentBytes(path, fiemapExtentsPerRequest)
	require.NoError(t, err)
	assert.Equal(t, int64(0), excl+shared, "no extents is an empty map")
}

// TestEveryAnalyzerUsesExclusiveSizeWithFakeKernel runs on any filesystem, so
// that the wiring of every analyzer is exercised where FIEMAP or reflinks are
// not available.
func TestEveryAnalyzerUsesExclusiveSizeWithFakeKernel(t *testing.T) {
	analyzers := append(parityAnalyzers(),
		parityAnalyzer{name: "parallel", analyzer: func(*testing.T) Analyzer { return CreateAnalyzer() }},
		parityAnalyzer{name: "top-dir", analyzer: func(*testing.T) Analyzer { return CreateTopDirAnalyzer() }},
	)
	for _, tc := range analyzers {
		t.Run(tc.name, func(t *testing.T) {
			dir := exclusiveTestDir(t)
			sub := filepath.Join(dir, "sub")
			require.NoError(t, os.Mkdir(sub, 0o755))
			writeSynced(t, filepath.Join(dir, "a"), make([]byte, 8192))
			writeSynced(t, filepath.Join(sub, "b"), make([]byte, 8192))
			fakeFiemap(t, mapExtents(
				fiemapExtent{length: 4096},
				fiemapExtent{logical: 4096, length: 12288, flags: fiemapExtentShared | fiemapExtentLast},
			))

			a := tc.analyzer(t)
			root := a.AnalyzeDir(context.Background(), dir, Options{ExclusiveSize: true})
			root.UpdateStats(false)

			assert.Equal(t, 2*4096+dirOwnUsage(dir)+dirOwnUsage(sub), root.GetUsage())
			if tc.name == "top-dir" {
				return // keeps no shared bytes, it is never exported
			}
			for child := range root.GetFiles(fs.SortByName, fs.SortAsc) {
				if file, ok := child.(*File); ok {
					assert.Equal(t, int64(12288), file.SharedUsage, "shared bytes of %s", child.GetName())
				}
			}
		})
	}
}

func TestExclusiveUsageFallsBackOnIoctlError(t *testing.T) {
	path := filepath.Join(exclusiveTestDir(t), "f")
	writeSynced(t, path, make([]byte, 8192))
	info, err := os.Stat(path)
	require.NoError(t, err)
	fakeFiemap(t, func(*fiemapRequest) syscall.Errno { return syscall.EOPNOTSUPP })

	excl, shared := new(extentMeter).measure(path, info, 8192)
	assert.Equal(t, int64(8192), excl)
	assert.Equal(t, int64(0), shared)
}

func TestExtentMeterRemembersFilesystemsWithoutFiemap(t *testing.T) {
	dir := exclusiveTestDir(t)
	var infos []os.FileInfo
	for _, name := range []string{"a", "b"} {
		path := filepath.Join(dir, name)
		writeSynced(t, path, make([]byte, 8192))
		info, err := os.Stat(path)
		require.NoError(t, err)
		infos = append(infos, info)
	}

	for _, errno := range []syscall.Errno{syscall.EOPNOTSUPP, syscall.ENOTTY} {
		calls := fakeFiemap(t, func(*fiemapRequest) syscall.Errno { return errno })
		m := &extentMeter{}
		usage, _ := m.measure(filepath.Join(dir, "a"), infos[0], 8192)
		assert.Equal(t, int64(8192), usage)
		usage, _ = m.measure(filepath.Join(dir, "b"), infos[1], 8192)
		assert.Equal(t, int64(8192), usage)
		assert.Equal(t, 1, *calls, "%v: the second file is not asked again", errno)
	}

	// any other error is about the file, not the filesystem
	calls := fakeFiemap(t, func(*fiemapRequest) syscall.Errno { return syscall.EIO })
	m := &extentMeter{}
	m.measure(filepath.Join(dir, "a"), infos[0], 8192)
	m.measure(filepath.Join(dir, "b"), infos[1], 8192)
	assert.Equal(t, 2, *calls)
}

func TestAnalyzerMeasureSharedUsageWithFakeKernel(t *testing.T) {
	path := filepath.Join(exclusiveTestDir(t), "f")
	writeSynced(t, path, make([]byte, 8192))
	info, err := os.Stat(path)
	require.NoError(t, err)
	fakeFiemap(t, mapExtents(
		fiemapExtent{length: 4096},
		fiemapExtent{logical: 4096, length: 12288, flags: fiemapExtentShared | fiemapExtentLast},
	))

	off := &scan{}
	usage, shared := off.measureSharedUsage(path, info, 8192)
	assert.Equal(t, int64(8192), usage, "disabled keeps st_blocks")
	assert.Equal(t, int64(0), shared)

	on := &scan{extents: &extentMeter{}}
	usage, shared = on.measureSharedUsage(path, info, 8192)
	assert.Equal(t, int64(4096), usage)
	assert.Equal(t, int64(12288), shared)
}
