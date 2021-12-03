//go:build freebsd || darwin
// +build freebsd darwin

package device

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"
)

// BSDDevicesInfoGetter returns info for Darwin devices
type BSDDevicesInfoGetter struct {
	MountCmd string
}

// Getter is current instance of DevicesInfoGetter
var Getter DevicesInfoGetter = BSDDevicesInfoGetter{MountCmd: "/sbin/mount"}

// GetMounts returns all mounted filesystems from output of /sbin/mount
func (t BSDDevicesInfoGetter) GetMounts() (Devices, error) {
	out, err := exec.Command(t.MountCmd).Output()
	if err != nil {
		return nil, err
	}

	rdr := bytes.NewReader(out)

	return readMountOutput(rdr)
}

// GetDevicesInfo returns result of GetMounts with usage info about mounted devices (by calling Statfs syscall)
func (t BSDDevicesInfoGetter) GetDevicesInfo() (Devices, error) {
	mounts, err := t.GetMounts()
	if err != nil {
		return nil, err
	}

	return processMounts(mounts, false)
}

func readMountOutput(rdr io.Reader) (Devices, error) {
	mounts := Devices{}

	scanner := bufio.NewScanner(rdr)
	for scanner.Scan() {
		line := scanner.Text()

		re := regexp.MustCompile("^(.*) on (/.*) \\(([^)]+)\\)$")
		parts := re.FindAllStringSubmatch(line, -1)

		if len(parts) < 1 {
			return nil, errors.New("Cannot parse mount output")
		}

		fstype := strings.TrimSpace(strings.Split(parts[0][3], ",")[0])

		device := &Device{
			Name:       parts[0][1],
			MountPoint: parts[0][2],
			Fstype:     fstype,
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
		if strings.HasPrefix(mount.Name, "/dev") || mount.Fstype == "zfs" {
			info := &unix.Statfs_t{}
			err := unix.Statfs(mount.MountPoint, info)
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
