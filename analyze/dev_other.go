// +build windows darwin openbsd freebsd netbsd plan9

package analyze

import "errors"

// OtherDevicesInfoGetter retruns info for other devices
type OtherDevicesInfoGetter struct{}

// GetDevicesInfo returns usage info about mounted devices (by calling Statfs syscall)
func (t OtherDevicesInfoGetter) GetDevicesInfo() ([]*Device, error) {
	return nil, errors.New("Only Linux platform is supported for listing devices")
}
