//go:build windows || plan9

package device

import "errors"

// OtherDevicesInfoGetter returns info for other devices
type OtherDevicesInfoGetter struct{}

// Getter is current instance of DevicesInfoGetter
var Getter DevicesInfoGetter = OtherDevicesInfoGetter{}

// GetDevicesInfo returns result of GetMounts with usage info about mounted devices
func (t OtherDevicesInfoGetter) GetDevicesInfo() (devices Devices, err error) {
	return nil, errors.New("Only Linux platform is supported for listing devices")
}

// GetMounts returns all mounted filesystems
func (t OtherDevicesInfoGetter) GetMounts() (devices Devices, err error) {
	return nil, errors.New("Only Linux platform is supported for listing mount points")
}
