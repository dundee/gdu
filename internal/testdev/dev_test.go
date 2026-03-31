package testdev

import (
	"testing"

	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/stretchr/testify/assert"
)

// Compile-time check that DevicesInfoGetterMock implements device.DevicesInfoGetter
var _ device.DevicesInfoGetter = DevicesInfoGetterMock{}

func TestGetDevicesInfo(t *testing.T) {
	devices := device.Devices{
		{
			Name:       "/dev/sda1",
			MountPoint: "/",
			Size:       1e12,
			Free:       5e11,
		},
		{
			Name:       "/dev/sda2",
			MountPoint: "/home",
			Size:       2e12,
			Free:       1e12,
		},
	}
	mock := DevicesInfoGetterMock{Devices: devices}

	result, err := mock.GetDevicesInfo()

	assert.NoError(t, err)
	assert.Equal(t, devices, result)
	assert.Len(t, result, 2)
	assert.Equal(t, "/dev/sda1", result[0].Name)
	assert.Equal(t, "/home", result[1].MountPoint)
}

func TestGetMounts(t *testing.T) {
	devices := device.Devices{
		{
			Name:       "/dev/sda1",
			MountPoint: "/",
			Size:       1e12,
			Free:       5e11,
		},
	}
	mock := DevicesInfoGetterMock{Devices: devices}

	result, err := mock.GetMounts()

	assert.NoError(t, err)
	assert.Equal(t, devices, result)
	assert.Len(t, result, 1)
}

func TestGetDevicesInfoEmpty(t *testing.T) {
	mock := DevicesInfoGetterMock{}

	result, err := mock.GetDevicesInfo()

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestGetMountsEmpty(t *testing.T) {
	mock := DevicesInfoGetterMock{}

	result, err := mock.GetMounts()

	assert.NoError(t, err)
	assert.Nil(t, result)
}
