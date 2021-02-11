package testdev

import "github.com/dundee/gdu/device"

// DevicesInfoGetterMock is mock of DevicesInfoGetter
type DevicesInfoGetterMock struct {
	Devices []*device.Device
}

// GetDevicesInfo returns mocked devices
func (t DevicesInfoGetterMock) GetDevicesInfo() ([]*device.Device, error) {
	return t.Devices, nil
}

// GetMounts returns all mounted filesystems from /proc/mounts
func (t DevicesInfoGetterMock) GetMounts() ([]*device.Device, error) {
	return t.Devices, nil
}
