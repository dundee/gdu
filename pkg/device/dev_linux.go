package device

import (
	"bufio"
	"io"
	"os"
	"strings"
	"syscall"
)

// LinuxDevicesInfoGetter retruns info for Linux devices
type LinuxDevicesInfoGetter struct {
	MountsPath string
}

// Getter is current instance of DevicesInfoGetter
var Getter DevicesInfoGetter = LinuxDevicesInfoGetter{MountsPath: "/proc/mounts"}

// GetMounts returns all mounted filesystems from /proc/mounts
func (t LinuxDevicesInfoGetter) GetMounts() (Devices, error) {
	file, err := os.Open(t.MountsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readMountsFile(file)
}

// GetDevicesInfo returns result of GetMounts with usage info about mounted devices (by calling Statfs syscall)
func (t LinuxDevicesInfoGetter) GetDevicesInfo() (Devices, error) {
	mounts, err := t.GetMounts()
	if err != nil {
		return nil, err
	}

	return processMounts(mounts, false)
}

func readMountsFile(file io.Reader) (Devices, error) {
	mounts := Devices{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		device := &Device{
			Name:       parts[0],
			MountPoint: parts[1],
			Fstype:     parts[2],
		}
		mounts = append(mounts, device)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}

func processMounts(mounts Devices, ignoreErrors bool) (Devices, error) {
	devices := Devices{}

	for _, mount := range mounts {
		if strings.Contains(mount.MountPoint, "/snap/") {
			continue
		}

		if strings.HasPrefix(mount.Name, "/dev") || mount.Fstype == "zfs" {
			info := &syscall.Statfs_t{}
			err := syscall.Statfs(mount.MountPoint, info)
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
