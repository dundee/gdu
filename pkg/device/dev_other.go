//go:build windows || plan9
// +build windows plan9

package device

import "errors"

// OtherDevicesInfoGetter retruns info for other devices
type OtherDevicesInfoGetter struct{}

// Getter is current instance of DevicesInfoGetter
var Getter DevicesInfoGetter = OtherDevicesInfoGetter{}

// GetDevicesInfo returns result of GetMounts with usage info about mounted devices
func (t OtherDevicesInfoGetter) GetDevicesInfo() (Devices, error) {
	return nil, errors.New("Only Linux platform is supported for listing devices")
}

// GetMounts returns all mounted filesystems
func (t OtherDevicesInfoGetter) GetMounts() (Devices, error) {
	return nil, errors.New("Only Linux platform is supported for listing mount points")
}
