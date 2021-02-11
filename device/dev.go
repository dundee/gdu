package device

// Device struct
type Device struct {
	Name       string
	MountPoint string
	Fstype     string
	Size       int64
	Free       int64
}

// DevicesInfoGetter is type for GetDevicesInfo function
type DevicesInfoGetter interface {
	GetMounts() (Devices, error)
	GetDevicesInfo() (Devices, error)
}

// Devices if slice of Device items
type Devices []*Device
}
