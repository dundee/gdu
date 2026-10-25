//go:build windows || plan9

package analyze

import (
	"os"
	"syscall"
	"time"
)

// Checking to see if this file is an "online only" OneDrive file, as they don't take any disk space.
// https://learn.microsoft.com/en-us/windows/win32/fileio/file-attribute-constants
const (
	fileAttributeRecallOnOpen       = 0x100000
	fileAttributeRecallOnDataAccess = 0x400000
)

func isRecallFile(info os.FileInfo) bool {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return data.FileAttributes&(fileAttributeRecallOnOpen|fileAttributeRecallOnDataAccess) != 0
	}
	return false
}

func getPlatformSpecificUsageAndMli(info os.FileInfo) (usage int64, ino uint64) {
	if isRecallFile(info) {
		return 0, 0
	}
	return info.Size(), 0 // No block info on Windows, use apparent size
}

func setPlatformSpecificAttrs(file *File, f os.FileInfo) {
	if stat, ok := f.Sys().(*syscall.Win32FileAttributeData); ok {
		file.Mtime = time.Unix(0, stat.LastWriteTime.Nanoseconds())
	}
	if isRecallFile(f) {
		file.Usage = 0
	} else {
		file.Usage = f.Size() // No block info on Windows, use apparent size
	}
}

func setDirPlatformSpecificAttrs(dir *Dir, path string) {
	stat, err := os.Stat(path)
	if err != nil {
		return
	}
	dir.Mtime = stat.ModTime()
	dir.OwnUsage = dirOwnUsage(path)
}

// dirOwnUsage returns 0: these platforms expose no block allocation for a
// directory, so gdu charges it nothing rather than guessing a block.
func dirOwnUsage(path string) int64 {
	return 0
}

// getSyscallStats extracts usage and inode info from os.FileInfo using syscall
func getSyscallStats(info os.FileInfo) (usage int64, mli uint64) {
	if isRecallFile(info) {
		return 0, 0
	}
	usage = info.Size()
	return
}

