//go:build darwin || netbsd || freebsd
// +build darwin netbsd freebsd

package analyze

import (
	"os"
	"syscall"
	"time"
)

const devBSize = 512

func setPlatformSpecificAttrs(file *File, f os.FileInfo) {
	if stat, ok := f.Sys().(*syscall.Stat_t); ok {
		file.Usage = stat.Blocks * devBSize
		file.Mtime = time.Unix(int64(stat.Mtimespec.Sec), int64(stat.Mtimespec.Nsec))

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

	dir.Mtime = time.Unix(int64(stat.Mtimespec.Sec), int64(stat.Mtimespec.Nsec))
}
