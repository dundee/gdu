//go:build linux || openbsd

package analyze

import (
	"os"
	"syscall"
	"time"
)

const devBSize = 512

func getPlatformSpecificUsageAndMli(info os.FileInfo) (usage int64, ino uint64) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if stat.Nlink > 1 {
			ino = stat.Ino
		}

		return stat.Blocks * devBSize, ino
	}
	return 0, 0
}

func setPlatformSpecificAttrs(file *File, f os.FileInfo) {
	if stat, ok := f.Sys().(*syscall.Stat_t); ok {
		file.Usage = stat.Blocks * devBSize
		file.Mtime = time.Unix(int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec))

		if stat.Nlink > 1 {
			file.Mli = stat.Ino
		}
	}
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

// getSyscallStats extracts usage and inode info from os.FileInfo using syscall
func getSyscallStats(info os.FileInfo) (usage int64, mli uint64) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		usage = stat.Blocks * 512 // 512-byte blocks
		if stat.Nlink > 1 {
			mli = stat.Ino
		}
	} else {
		usage = info.Size()
	}
	return
}
