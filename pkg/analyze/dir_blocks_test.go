//go:build linux || openbsd || darwin || netbsd || freebsd

package analyze

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

// wantUsage sums what du would report for a path: the block allocation of the
// path itself, taken from stat, never assumed.
func wantUsage(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Lstat(path)
	assert.NoError(t, err)
	return info.Sys().(*syscall.Stat_t).Blocks * 512
}

// TestDirOwnBlocksAreCounted pins issue #393: a directory's disk usage must
// account for its own st_blocks, not a fixed assumption of one block.
//
// gdu previously hardcoded 4096 for every directory, so on XFS -- where a small
// directory can live inline in the inode and stat reports Blocks: 0 -- it
// reported 4.0 KiB where du reports 0. The replacement then charged directories
// nothing at all, so a directory that really did allocate a block was dropped
// from the total. Reading st_blocks agrees with du on both.
func TestDirOwnBlocksAreCounted(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "d")
	assert.NoError(t, os.Mkdir(dir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("x"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "b"), []byte("y"), 0o600))

	// du charges the directory's own block plus each file's.
	want := wantUsage(t, dir) + wantUsage(t, filepath.Join(dir, "a")) +
		wantUsage(t, filepath.Join(dir, "b"))

	analyzer := CreateSeqAnalyzer()
	scanned := analyzer.AnalyzeDir(root, func(_, _ string) bool { return false }, nil).(*Dir)
	analyzer.GetDone().Wait()
	scanned.UpdateStats(make(fs.HardLinkedItems))

	subdir := scanned.Files[0].(*Dir)
	assert.Equal(t, want, subdir.GetUsage(),
		"directory usage must include its own st_blocks, not assume a block count")
}

// TestEmptyDirOwnBlocksAreCounted is the #393 case proper: an inline directory
// with zero blocks must report zero, not one block.
func TestEmptyDirOwnBlocksAreCounted(t *testing.T) {
	root := t.TempDir()

	analyzer := CreateSeqAnalyzer()
	scanned := analyzer.AnalyzeDir(root, func(_, _ string) bool { return false }, nil).(*Dir)
	analyzer.GetDone().Wait()
	scanned.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, wantUsage(t, root), scanned.GetUsage(),
		"directory usage must equal its own st_blocks*512; 0 blocks must report 0")
}
