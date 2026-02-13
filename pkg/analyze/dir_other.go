//go:build windows || plan9

package analyze

import (
	"os"
	"syscall"
	"time"
)

func setPlatformSpecificAttrs(file *File, f os.FileInfo) {
	stat := f.Sys().(*syscall.Win32FileAttributeData)
	file.Mtime = time.Unix(0, stat.LastWriteTime.Nanoseconds())
	file.Usage = f.Size() // No block info on Windows, use apparent size
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
