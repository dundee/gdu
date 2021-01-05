// +build windows darwin linux,arm

package analyze

import "errors"

// GetDevicesInfo returns usage info about mounted devices (by calling Statfs syscall)
func GetDevicesInfo(mountsPath string) ([]*Device, error) {
	return nil, errors.New("Only Linux platform is supported for listing devices")
}
