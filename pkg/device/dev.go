package device

import (
	"path/filepath"
	"strings"
)

// Device struct
type Device struct {
	Name       string
	MountPoint string
	Fstype     string
	// FilesystemID is the Linux device number, or nil when unavailable.
	FilesystemID *uint64
	Size         int64
	Free         int64
}

// GetUsage returns used size of device
func (d Device) GetUsage() int64 {
	return d.Size - d.Free
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
	return f[i].GetUsage() < f[j].GetUsage()
}

// ByName sorts devices by device name
type ByName Devices

func (f ByName) Len() int      { return len(f) }
func (f ByName) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ByName) Less(i, j int) bool {
	return f[i].Name < f[j].Name
}

// GetNestedMountpointsPaths returns nested mount points on other or unknown filesystems.
// Mount points are real paths, so they are matched against path with symlinks resolved,
// and returned under path, where the scan finds them
// (e.g. /Volumes/Macintosh HD/dev when scanning /Volumes/Macintosh HD -> /).
func GetNestedMountpointsPaths(path string, mounts Devices) []string {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}
	paths := make([]string, 0, len(mounts))
	filesystemID, known := getFilesystemID(realPath)

	for _, mount := range mounts {
		relative, nested := nestedPath(realPath, mount.MountPoint)
		if !nested {
			alias, ok := getMountPointAlias(mount.MountPoint)
			if !ok {
				continue
			}
			if relative, nested = nestedPath(realPath, alias); !nested {
				continue
			}
		}
		if known && mount.FilesystemID != nil && filesystemID == *mount.FilesystemID {
			continue
		}
		paths = append(paths, filepath.Join(path, relative))
	}
	return paths
}

// nestedPath returns path relative to basePath, and whether path is nested inside basePath.
func nestedPath(basePath, path string) (string, bool) {
	relative, err := filepath.Rel(basePath, path)
	return relative, err == nil && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
