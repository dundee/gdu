// +build !windows
// +build !plan9

package analyze

import (
	"os"
	"syscall"
)

const devBSize = 512

func getUsage(f os.FileInfo) int64 {
	var usage int64 = 0

	switch stat := f.Sys().(type) {
	case *syscall.Stat_t:
		usage = stat.Blocks * devBSize
	}
	return usage
}
