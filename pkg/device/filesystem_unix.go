//go:build !windows && !plan9

package device

import "golang.org/x/sys/unix"

// GetFilesystemID returns the device number of the filesystem the path lives on
func GetFilesystemID(path string) (uint64, bool) {
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		return 0, false
	}
	return uint64(stat.Dev), true
}
