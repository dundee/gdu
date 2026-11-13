//go:build darwin

package analyze

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// exclusiveTestDir is the test's temporary directory, which is on the system
// volume's APFS on any current Mac; the tests that need clones skip elsewhere.
func exclusiveTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
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
	info, err := os.Lstat(path)
	require.NoError(t, err)
	return info.Sys().(*syscall.Stat_t).Blocks * 512
}

// measure skips the test when the filesystem does not report private size.
func measure(t *testing.T, path string) int64 {
	t.Helper()
	n, err := privateSize(path)
	if errors.Is(err, errPrivateSizeNotSupported) {
		t.Skip("the filesystem does not report private size")
	}
	require.NoError(t, err)
	return n
}

// makeClone writes src and clones it to dst with clonefile(2), skipping the
// test where the filesystem cannot clone.
func makeClone(t *testing.T, dir string, size int) (src, dst string) {
	t.Helper()
	src = filepath.Join(dir, "src")
	dst = filepath.Join(dir, "dst")
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i%251) + 1
	}
	writeSynced(t, src, data)
	err := unix.Clonefile(src, dst, 0)
	if errors.Is(err, unix.ENOTSUP) {
		t.Skipf("clonefile is not supported here: %v", err)
	}
	require.NoError(t, err)
	return src, dst
}

func rewriteStart(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteAt([]byte("changed"), 0)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())
}

func TestPrivateSizePlainFile(t *testing.T) {
	path := filepath.Join(exclusiveTestDir(t), "plain")
	writeSynced(t, path, make([]byte, 1<<20))

	// Nothing is shared, so it matches what the filesystem allocated.
	assert.Equal(t, allocated(t, path), measure(t, path))
}

func TestPrivateSizeClone(t *testing.T) {
	const size = 1 << 20
	src, dst := makeClone(t, exclusiveTestDir(t), size)

	assert.Equal(t, int64(0), measure(t, src))
	assert.Equal(t, int64(0), measure(t, dst))
	for _, p := range []string{src, dst} {
		assert.GreaterOrEqual(t, allocated(t, p), int64(size), "st_blocks counts the clone in full")
	}

	// Rewriting part of one copy gives both blocks of their own.
	rewriteStart(t, dst)
	excl := measure(t, dst)
	assert.Greater(t, excl, int64(0))
	assert.Less(t, excl, int64(size), "most of the file is still shared")

	// Once the other copy is gone, all of the file is its own.
	require.NoError(t, os.Remove(dst))
	assert.Equal(t, allocated(t, src), measure(t, src))
}

func TestPrivateSizeErrors(t *testing.T) {
	_, err := privateSize(filepath.Join(exclusiveTestDir(t), "missing"))
	assert.ErrorIs(t, err, syscall.ENOENT)
}

func TestExclusiveUsageSplitsClone(t *testing.T) {
	const size = 1 << 20
	_, dst := makeClone(t, exclusiveTestDir(t), size)
	rewriteStart(t, dst)
	info, err := os.Lstat(dst)
	require.NoError(t, err)
	usage := allocated(t, dst)

	excl, shared := new(extentMeter).measure(dst, info, usage)
	assert.Equal(t, measure(t, dst), excl)
	assert.Equal(t, usage, excl+shared, "every allocated byte is one or the other")
}

func TestExclusiveUsageKeepsUsageWhenNotMeasurable(t *testing.T) {
	dir := exclusiveTestDir(t)

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	got, shared := new(extentMeter).measure(dir, dirInfo, 4096)
	assert.Equal(t, int64(4096), got, "directories are not measured")
	assert.Equal(t, int64(0), shared)

	path := filepath.Join(dir, "f")
	writeSynced(t, path, make([]byte, 8192))
	info, err := os.Stat(path)
	require.NoError(t, err)
	got, shared = new(extentMeter).measure(path, info, 0)
	assert.Equal(t, int64(0), got, "nothing allocated, nothing to measure")
	assert.Equal(t, int64(0), shared)
}

// fakeGetattrlist replaces getattrlist(2) with the given replies, one per
// call, and returns a pointer to the number of calls made.
func fakeGetattrlist(t *testing.T, replies ...func(reply *privateSizeReply) error) *int {
	t.Helper()
	orig := getattrlist
	t.Cleanup(func() { getattrlist = orig })
	calls := 0
	getattrlist = func(_ string, _ *unix.Attrlist, reply *privateSizeReply) error {
		r := replies[min(calls, len(replies)-1)]
		calls++
		return r(reply)
	}
	return &calls
}

func privateReply(private int64) func(reply *privateSizeReply) error {
	return func(reply *privateSizeReply) error {
		reply.returned[4] = attrCmnextPrivateSize
		reply.private = private
		return nil
	}
}

func TestPrivateSizeRetriesInterruptedCall(t *testing.T) {
	calls := fakeGetattrlist(t, func(*privateSizeReply) error { return syscall.EINTR }, privateReply(4096))

	n, err := privateSize("any")
	require.NoError(t, err)
	assert.Equal(t, int64(4096), n)
	assert.Equal(t, 2, *calls)
}

// statInfo is a regular file's FileInfo with the given stat fields, for what
// a test cannot make the filesystem produce.
type statInfo struct {
	stat syscall.Stat_t
}

func (s statInfo) Name() string       { return "f" }
func (s statInfo) Size() int64        { return 8192 }
func (s statInfo) Mode() os.FileMode  { return 0o644 }
func (s statInfo) ModTime() time.Time { return time.Time{} }
func (s statInfo) IsDir() bool        { return false }
func (s statInfo) Sys() any           { return &s.stat }

func TestExclusiveUsageKeepsCompressedFiles(t *testing.T) {
	calls := fakeGetattrlist(t, privateReply(0))

	info := statInfo{stat: syscall.Stat_t{Flags: unix.UF_COMPRESSED}}
	excl, shared := new(extentMeter).measure("f", info, 8192)
	assert.Equal(t, int64(8192), excl, "a compressed file's private size misses its data")
	assert.Equal(t, int64(0), shared)
	assert.Equal(t, 0, *calls, "a compressed file is not asked")
}

func TestExclusiveUsageKeepsUsageWithinBlocks(t *testing.T) {
	fakeGetattrlist(t, privateReply(16384))

	excl, shared := new(extentMeter).measure("f", statInfo{}, 8192)
	assert.Equal(t, int64(8192), excl)
	assert.Equal(t, int64(0), shared)
}

func TestExclusiveUsageFallsBackPerFile(t *testing.T) {
	same := statInfo{stat: syscall.Stat_t{Dev: 7}}
	notReturned := func(*privateSizeReply) error { return nil }
	for _, first := range []func(*privateSizeReply) error{
		notReturned,
		func(*privateSizeReply) error { return syscall.EACCES },
	} {
		calls := fakeGetattrlist(t, first, privateReply(4096))
		m := &extentMeter{}
		usage, shared := m.measure("a", same, 8192)
		assert.Equal(t, int64(8192), usage, "the file keeps its blocks")
		assert.Equal(t, int64(0), shared)
		usage, _ = m.measure("b", same, 8192)
		assert.Equal(t, int64(4096), usage, "the next file on the device is still measured")
		assert.Equal(t, 2, *calls)
	}
}

func TestExclusiveUsageFollowsSymlinks(t *testing.T) {
	dir := exclusiveTestDir(t)
	src := filepath.Join(dir, "src")
	writeSynced(t, src, make([]byte, 1<<20))
	measure(t, src)
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(src, link))
	target, err := os.Stat(link) // what --follow-symlinks hands over
	require.NoError(t, err)

	// The link itself owns next to nothing; its target owns all its blocks.
	excl, shared := new(extentMeter).measure(link, target, allocated(t, src))
	assert.Equal(t, allocated(t, src), excl)
	assert.Equal(t, int64(0), shared)
}

func TestAnalyzerMeasureSharedUsageWithFakeKernel(t *testing.T) {
	fakeGetattrlist(t, privateReply(4096))

	off := &scan{}
	usage, shared := off.measureSharedUsage("f", statInfo{}, 8192)
	assert.Equal(t, int64(8192), usage, "disabled keeps st_blocks")
	assert.Equal(t, int64(0), shared)

	on := &scan{extents: &extentMeter{}}
	usage, shared = on.measureSharedUsage("f", statInfo{}, 8192)
	assert.Equal(t, int64(4096), usage)
	assert.Equal(t, int64(4096), shared)
}

func TestEveryAnalyzerReportsExclusiveSizeOfClones(t *testing.T) {
	analyzers := append(parityAnalyzers(),
		parityAnalyzer{name: "parallel", analyzer: func(*testing.T) common.Analyzer { return CreateAnalyzer() }},
		parityAnalyzer{name: "top-dir", analyzer: func(*testing.T) common.Analyzer { return CreateTopDirAnalyzer() }},
	)
	const size = 1 << 20
	for _, tc := range analyzers {
		t.Run(tc.name, func(t *testing.T) {
			dir := exclusiveTestDir(t)
			src, _ := makeClone(t, dir, size)
			measure(t, src)

			scan := func(exclusive bool) fs.Item {
				a := tc.analyzer(t)
				root := a.AnalyzeDir(context.Background(), dir, common.ScanOptions{ExclusiveSize: exclusive})
				root.UpdateStats(make(fs.HardLinkedItems))
				return root
			}
			normal := scan(false).GetUsage() - dirOwnUsage(dir)
			root := scan(true)
			exclusive := root.GetUsage() - dirOwnUsage(dir)

			assert.GreaterOrEqual(t, normal, int64(2*size), "block usage counts the clone per file")
			assert.Equal(t, int64(0), exclusive, "everything is shared")
			if tc.name == "top-dir" {
				return // keeps no shared bytes, it is never exported
			}
			for child := range root.GetFiles(fs.SortByName, fs.SortAsc) {
				if file, ok := child.(*File); ok {
					assert.GreaterOrEqual(t, file.SharedUsage, int64(size), "shared bytes of %s", child.GetName())
				}
			}
		})
	}
}
