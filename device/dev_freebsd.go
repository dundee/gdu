package device

import (
	"bufio"
	"bytes"
	"os/exec"
	"io"
	"strings"
	"syscall"
)

// FreeBSDDevicesInfoGetter returns info for FreeBSD devices
type FreeBSDDevicesInfoGetter struct {
	MountsPath string
}

// Getter is current instance of DevicesInfoGetter
var Getter DevicesInfoGetter = FreeBSDDevicesInfoGetter{}

// GetMounts returns all mounted filesystems from /proc/mounts
func (t FreeBSDDevicesInfoGetter) GetMounts() (Devices, error) {
	out, err := exec.Command("/sbin/mount").Output()
	if err != nil {
		return nil, err
	}

	rdr := bytes.NewReader(out)

	return readMountOutput(rdr)
}

// GetDevicesInfo returns result of GetMounts with usage info about mounted devices (by calling Statfs syscall)
func (t FreeBSDDevicesInfoGetter) GetDevicesInfo() (Devices, error) {
	mounts, err := t.GetMounts()
	if err != nil {
		return nil, err
	}

	return processMounts(mounts)
}

func readMountOutput(rdr io.Reader) (Devices, error) {
	mounts := Devices{}

	scanner := bufio.NewScanner(rdr)
	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.Fields(line)

		fstype := parts[3][1:len(parts[3])-1]

		device := &Device{
			Name:       parts[0],
			MountPoint: parts[2],
			Fstype:     fstype,
		}
		mounts = append(mounts, device)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}

func processMounts(mounts Devices) (Devices, error) {
	devices := Devices{}

	for _, mount := range mounts {
		if strings.HasPrefix(mount.Name, "/dev") || mount.Fstype == "zfs" {
			info := &syscall.Statfs_t{}
			syscall.Statfs(mount.MountPoint, info)

			mount.Size = int64(info.Bsize) * int64(info.Blocks)
			mount.Free = int64(info.Bsize) * int64(info.Bavail)

			devices = append(devices, mount)
		}
	}

	return devices, nil
}
