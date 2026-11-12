package analyze

import (
	"os"
)

func getPlatformSpecificUsageAndMli(info os.FileInfo, _ string) (usage int64, ino uint64) {
	return info.Size(), 0 // No block info on Plan9, use apparent size
}

func setPlatformSpecificAttrs(file *File, f os.FileInfo, path string) {
	file.Mtime = f.ModTime()
	file.Usage, _ = getPlatformSpecificUsageAndMli(f, path)
}

// setDirPlatformSpecificAttrs fills the attributes of a directory from the info the caller already has
func setDirPlatformSpecificAttrs(dir *Dir, info os.FileInfo) {
	dir.Mtime = info.ModTime()
}

// getInode returns 0, inode numbers are not used on this platform
func getInode(os.FileInfo) uint64 {
	return 0
}

// dirOwnUsage returns 0: Plan9 exposes no block allocation for a
// directory, so gdu charges it nothing rather than guessing a block.
func dirOwnUsage(string) int64 {
	return 0
}

// dirInfoOwnUsage is dirOwnUsage for a directory that has already been stat'ed.
func dirInfoOwnUsage(os.FileInfo) int64 {
	return 0
}

// getSyscallStats extracts usage and inode info from os.FileInfo
func getSyscallStats(info os.FileInfo, path string) (usage int64, mli uint64) {
	return getPlatformSpecificUsageAndMli(info, path)
}
