package testdev

import "github.com/dundee/gdu/v4/pkg/device"

// DevicesInfoGetterMock is mock of DevicesInfoGetter
type DevicesInfoGetterMock struct {
	Devices device.Devices
}

// GetDevicesInfo returns mocked devices
func (t DevicesInfoGetterMock) GetDevicesInfo() (device.Devices, error) {
	return t.Devices, nil
}

// GetMounts returns all mounted filesystems from /proc/mounts
func (t DevicesInfoGetterMock) GetMounts() (device.Devices, error) {
	return t.Devices, nil
}
