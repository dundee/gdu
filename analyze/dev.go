package analyze

// Device struct
type Device struct {
	Name       string
	MountPoint string
	Size       int64
	Free       int64
}

// DevicesInfoGetter is type for GetDevicesInfo function
type DevicesInfoGetter interface {
	GetDevicesInfo() ([]*Device, error)
}
