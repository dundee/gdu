package device

import "strings"

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

// GetNestedMountpointsPaths returns paths of nested mount points
func GetNestedMountpointsPaths(path string, mounts Devices) []string {
	paths := make([]string, 0, len(mounts))

	for _, mount := range mounts {
		if strings.HasPrefix(mount.MountPoint, path) && mount.MountPoint != path {
			paths = append(paths, mount.MountPoint)
		}
	}
	return paths
}
