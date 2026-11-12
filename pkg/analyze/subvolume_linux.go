//go:build linux

package analyze

import (
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// btrfsSubvolumeInode is the inode number of every btrfs subvolume root
const btrfsSubvolumeInode = 256

// nestedBtrfsSubvolume reports whether the directory is the root of a btrfs subvolume that is not
// a mount point, which is what snapshots (snapper, timeshift, ...) look like. They share
// extents with the original, so counting them inflates usage far beyond the disk size.
// Mounted subvolumes (like a separate /home) are real data and are not matched.
// The inode comes from the info the caller already has, the filesystem is only asked for subvolume roots.
func nestedBtrfsSubvolume(path string, info os.FileInfo, mountPoints map[string]struct{}) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Ino != btrfsSubvolumeInode {
		return false
	}
	var fsInfo unix.Statfs_t
	if unix.Statfs(path, &fsInfo) != nil || fsInfo.Type != unix.BTRFS_SUPER_MAGIC {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	_, mounted := mountPoints[absPath]
	return !mounted
}
