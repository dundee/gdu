package testdev

import "github.com/dundee/gdu/v5/pkg/device"

// DevicesInfoGetterMock is mock of DevicesInfoGetter
type DevicesInfoGetterMock struct {
	Devices device.Devices
}

// GetDevicesInfo returns mocked devices
func (t DevicesInfoGetterMock) GetDevicesInfo() (devices device.Devices, err error) {
	return t.Devices, nil
}

// GetMounts returns all mounted filesystems from /proc/mounts
func (t DevicesInfoGetterMock) GetMounts() (devices device.Devices, err error) {
	return t.Devices, nil
}
