//go:build linux || openbsd

package analyze

import (
	"os"
	"syscall"
	"time"
)

const devBSize = 512

// fileStats returns the disk usage of the file described by info and, when
// the file has several hard links, its identity.
func fileStats(info os.FileInfo, _ string) (usage int64, link inodeKey) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, inodeKey{}
	}
	if stat.Nlink > 1 {
		// Dev is int32 on some platforms
		link = inodeKey{dev: uint64(stat.Dev), ino: stat.Ino}
	}
	return stat.Blocks * devBSize, link
}

func setDirPlatformSpecificAttrs(dir *Dir, path string) {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return
	}

	dir.Mtime = time.Unix(int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec))
	dir.OwnUsage = stat.Blocks * devBSize
}

// dirOwnUsage returns the disk usage of the blocks allocated to the directory
// itself. It is 0 when the filesystem stores the entries in the inode, e.g. a
// small directory on XFS.
func dirOwnUsage(path string) int64 {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return 0
	}
	return stat.Blocks * devBSize
}

// dirInfoOwnUsage is dirOwnUsage for a directory that has already been stat'ed.
func dirInfoOwnUsage(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Blocks * devBSize
	}
	return 0
}
