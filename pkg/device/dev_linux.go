package device

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mithrandie/go-file/v2"
	"golang.org/x/sys/unix"
)

// LinuxDevicesInfoGetter returns info for Linux devices
type LinuxDevicesInfoGetter struct {
	MountsPath string
}

// Getter is current instance of DevicesInfoGetter
var Getter DevicesInfoGetter = LinuxDevicesInfoGetter{MountsPath: "/proc/mounts"}

// GetMounts returns all mounted filesystems from /proc/mounts
func (t LinuxDevicesInfoGetter) GetMounts() (devices Devices, err error) {
	file, err = os.Open(t.MountsPath)
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
	if err = file.Close(); err != nil {
		return nil, err
	}
	return devices, nil
}

// GetDevicesInfo returns result of GetMounts with usage info about mounted devices (by calling Statfs syscall)
func (t LinuxDevicesInfoGetter) GetDevicesInfo() (devices Devices, err error) {
	mounts, err = t.GetMounts()
	if err != nil {
		return nil, err
	}

	return processMounts(mounts, false)
}

func readMountsFile(file io.Reader) (mounts Devices, err error) {
	mounts := Devices{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		device := &Device{
			Name:       parts[0],
			MountPoint: unescapeString(parts[1]),
			Fstype:     parts[2],
		}
		mounts = append(mounts, device)
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}

func processMounts(mounts Devices, ignoreErrors bool) (devices Devices, err error) {
	devices := Devices{}

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
