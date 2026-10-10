//go:build !linux

package device

import "errors"

func getFilesystemID(_ string) (uint64, bool) {
	return 0, false
}

// IsNestedBtrfsSubvolume is only implemented on Linux
func IsNestedBtrfsSubvolume(_ string, _ map[string]struct{}) bool {
	return false
}

// ReadExtents is only implemented on Linux
func ReadExtents(_ string) ([]Extent, error) {
	return nil, errors.ErrUnsupported
}
