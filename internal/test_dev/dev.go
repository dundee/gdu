package test_dev

import "github.com/dundee/gdu/device"

// DevicesInfoGetterMock is mock of DevicesInfoGetter
type DevicesInfoGetterMock struct {
	Devices []*device.Device
}

// GetDevicesInfo returns mocked devices
func (t DevicesInfoGetterMock) GetDevicesInfo() ([]*device.Device, error) {
	return t.Devices, nil
}
