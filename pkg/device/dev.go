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

// ByUsedSize sorts devices by used size
type ByUsedSize Devices

func (f ByUsedSize) Len() int      { return len(f) }
func (f ByUsedSize) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ByUsedSize) Less(i, j int) bool {
	return (f[i].Size - f[i].Free) > (f[j].Size - f[j].Free)
}

// ByName sorts devices by device name
type ByName Devices

func (f ByName) Len() int      { return len(f) }
func (f ByName) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ByName) Less(i, j int) bool {
	return f[i].Name > f[j].Name
}

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
