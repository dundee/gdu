package device

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// LinuxDevicesInfoGetter returns info for Linux devices
type LinuxDevicesInfoGetter struct {
	MountsPath string
}

// Getter is current instance of DevicesInfoGetter
var Getter DevicesInfoGetter = LinuxDevicesInfoGetter{MountsPath: "/proc/self/mountinfo"}

// GetMounts returns mounted filesystems from a mountinfo or legacy mounts file.
func (t LinuxDevicesInfoGetter) GetMounts() (devices Devices, err error) {
	file, err := os.Open(t.MountsPath)
	if err != nil {
		return nil, err
	}

	devices, err = readMountsFile(file)
	if err != nil {
		if cerr := file.Close(); cerr != nil {
			return nil, fmt.Errorf("%w; %s", err, cerr.Error())
		}
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return devices, nil
}

// GetDevicesInfo returns result of GetMounts with usage info about mounted devices (by calling Statfs syscall)
func (t LinuxDevicesInfoGetter) GetDevicesInfo() (devices Devices, err error) {
	mounts, err := t.GetMounts()
	if err != nil {
		return nil, err
	}

	return processMounts(mounts, false)
}

func readMountsFile(file io.Reader) (mounts Devices, err error) {
	mounts = Devices{}
	parents := make(map[*Device]string)
	byID := make(map[string]*Device)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		separator := slices.Index(parts, "-")
		switch {
		case separator >= 6 && len(parts) >= separator+4:
			mount := &Device{
				Name:         parts[separator+2],
				MountPoint:   unescapeString(parts[4]),
				Fstype:       parts[separator+1],
				FilesystemID: parseFilesystemID(parts[2]),
			}
			mounts = append(mounts, mount)
			parents[mount] = parts[1]
			byID[parts[0]] = mount
		case len(parts) >= 3 && strings.HasPrefix(parts[1], "/"):
			mounts = append(mounts, &Device{
				Name:       parts[0],
				MountPoint: unescapeString(parts[1]),
				Fstype:     parts[2],
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return visibleMounts(mounts, parents, byID), nil
}

func visibleMounts(mounts Devices, parents map[*Device]string, byID map[string]*Device) Devices {
	covered := make(map[*Device]bool)
	for mount, parentID := range parents {
		parent := byID[parentID]
		if parent != nil && parent != mount && parent.MountPoint == mount.MountPoint {
			covered[parent] = true
		}
	}
	return slices.DeleteFunc(mounts, func(mount *Device) bool {
		if covered[mount] {
			return true
		}
		// Bound malformed parent cycles in custom mount tables.
		for range len(parents) {
			parent := byID[parents[mount]]
			if parent == nil || parent == mount {
				break
			}
			if covered[parent] && parent.MountPoint != mount.MountPoint {
				return true
			}
			mount = parent
		}
		return false
	})
}

func processMounts(mounts Devices, ignoreErrors bool) (devices Devices, err error) {
	devices = Devices{}

	for _, mount := range mounts {
		if strings.Contains(mount.MountPoint, "/snap/") {
			continue
		}

		if strings.HasPrefix(mount.Name, "/dev") ||
			mount.Fstype == "zfs" ||
			mount.Fstype == "nfs" ||
			mount.Fstype == "nfs4" {
			info := &unix.Statfs_t{}
			err = unix.Statfs(mount.MountPoint, info)
			if err != nil && !ignoreErrors {
				return nil, err
			}

			mount.Size = int64(info.Bsize) * int64(info.Blocks)
			mount.Free = int64(info.Bsize) * int64(info.Bavail)

			devices = append(devices, mount)
		}
	}

	return devices, nil
}

func unescapeString(str string) string {
	return strings.ReplaceAll(str, "\\040", " ")
}

func getFilesystemID(path string) (uint64, bool) {
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		return 0, false
	}
	return uint64(stat.Dev), true
}

func parseFilesystemID(value string) *uint64 {
	majorText, minorText, found := strings.Cut(value, ":")
	if !found {
		return nil
	}
	major, err := strconv.ParseUint(majorText, 10, 32)
	if err != nil {
		return nil
	}
	minor, err := strconv.ParseUint(minorText, 10, 32)
	if err != nil {
		return nil
	}
	id := unix.Mkdev(uint32(major), uint32(minor))
	return &id
}
