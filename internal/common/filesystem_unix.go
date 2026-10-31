//go:build unix

package common

import "golang.org/x/sys/unix"

// getFilesystemID returns the device number of the filesystem the path lives on
var getFilesystemID = func(path string) (uint64, bool) {
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		return 0, false
	}
	return uint64(stat.Dev), true // nolint: unconvert // Why: Dev is not uint64 on all platforms
}
