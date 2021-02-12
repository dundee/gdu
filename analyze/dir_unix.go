// +build !windows
// +build !plan9

package analyze

import (
	"os"
	"syscall"
)

const devBSize = 512

func setPlatformSpecificAttrs(file *File, f os.FileInfo) {
	switch stat := f.Sys().(type) {
	case *syscall.Stat_t:
		file.Usage = stat.Blocks * devBSize

		if stat.Nlink > 1 {
			file.MutliLinkInode = stat.Ino
		}
	}
}
