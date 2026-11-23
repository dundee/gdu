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

// GetMounts returns no mount points, as listing them is not supported here.
// It does not fail, so that --no-cross can still be used. On Windows the scan
// does not enter junctions or mounted folders anyway: os.Lstat reports these
// mount point reparse points as irregular files, not as directories.
func (t OtherDevicesInfoGetter) GetMounts() (devices Devices, err error) {
	return Devices{}, nil
}
