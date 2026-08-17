//go:build windows || plan9

package analyze

import (
	"os"
	"syscall"
	"time"
)

func getPlatformSpecificUsageAndMli(info os.FileInfo) (usage int64, ino uint64) {
	return info.Size(), 0 // No block info on Windows, use apparent size
}

func setPlatformSpecificAttrs(file *File, f os.FileInfo) {
	stat := f.Sys().(*syscall.Win32FileAttributeData)
	file.Mtime = time.Unix(0, stat.LastWriteTime.Nanoseconds())
	file.Usage = f.Size() // No block info on Windows, use apparent size

	// Checking to see if this file is an "online only" OneDrive file, as they don't take any disk space.
	// https://learn.microsoft.com/en-us/windows/win32/fileio/file-attribute-constants
	const fileAttributeRecallOnOpen = 0x100000
	const fileAttributeRecallOnDataAccess = 0x400000
	if data, ok := f.Sys().(*syscall.Win32FileAttributeData); ok {
		if data.FileAttributes&(fileAttributeRecallOnOpen|fileAttributeRecallOnDataAccess) != 0 {
			file.Size = 0
		}
	}
}

func setDirPlatformSpecificAttrs(dir *Dir, path string) {
	stat, err := os.Stat(path)
	if err != nil {
		return
	}
	dir.Mtime = stat.ModTime()
}

// getSyscallStats extracts usage and inode info from os.FileInfo using syscall
func getSyscallStats(info os.FileInfo) (usage int64, mli uint64) {
	usage = info.Size()
	return
}
