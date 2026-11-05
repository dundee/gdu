//go:build !linux

package device

import "errors"

// ReadExtents is only implemented on Linux
func ReadExtents(_ string) ([]Extent, error) {
	return nil, errors.ErrUnsupported
}
