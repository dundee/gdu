//go:build windows || plan9
// +build windows plan9

package analyze

import (
	"os"
	"syscall"
	"time"
)

func setPlatformSpecificAttrs(file *File, f os.FileInfo) {
	stat := f.Sys().(*syscall.Win32FileAttributeData)
	file.Mtime = time.Unix(0, stat.LastWriteTime.Nanoseconds())
}

func setDirPlatformSpecificAttrs(dir *Dir, path string) {
	stat, err := os.Stat(path)
	if err != nil {
		return
	}
	dir.Mtime = stat.ModTime()
}
