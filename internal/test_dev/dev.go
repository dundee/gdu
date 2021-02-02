package test_dev

import "github.com/dundee/gdu/analyze"

// DevicesInfoGetterMock is mock of DevicesInfoGetter
type DevicesInfoGetterMock struct {
	Devices []*analyze.Device
}

// GetDevicesInfo returns mocked devices
func (t DevicesInfoGetterMock) GetDevicesInfo() ([]*analyze.Device, error) {
	return t.Devices, nil
}
