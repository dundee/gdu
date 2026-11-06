//go:build darwin || netbsd || freebsd

package analyze

import (
	"os"
	"syscall"
	"time"
)

const devBSize = 512

func getPlatformSpecificUsageAndMli(info os.FileInfo, _ string) (usage int64, ino uint64) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if stat.Nlink > 1 {
			ino = stat.Ino
		}
		return stat.Blocks * devBSize, ino
	}
	return 0, 0
}

func setPlatformSpecificAttrs(file *File, f os.FileInfo, _ string) {
	if stat, ok := f.Sys().(*syscall.Stat_t); ok {
		file.Usage = stat.Blocks * devBSize
		file.Mtime = time.Unix(int64(stat.Mtimespec.Sec), int64(stat.Mtimespec.Nsec))

		if stat.Nlink > 1 {
			file.Mli = stat.Ino
		}
	}
}

// setDirPlatformSpecificAttrs fills the attributes of a directory from the info the caller already has
func setDirPlatformSpecificAttrs(dir *Dir, info os.FileInfo) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		dir.Mtime = time.Unix(int64(stat.Mtimespec.Sec), int64(stat.Mtimespec.Nsec))
		dir.OwnUsage = stat.Blocks * devBSize
	}
}

// getInode returns the inode number of an already stat'ed item
func getInode(info os.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino
	}
	return 0
}

// dirOwnUsage returns the disk usage of the blocks allocated to the directory
// itself.
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
func getSyscallStats(info os.FileInfo, _ string) (usage int64, mli uint64) {
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
