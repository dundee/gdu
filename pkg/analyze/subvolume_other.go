//go:build !linux

package analyze

import "os"

// nestedBtrfsSubvolume is only implemented on Linux
func nestedBtrfsSubvolume(_ string, _ os.FileInfo, _ map[string]struct{}) bool {
	return false
}
