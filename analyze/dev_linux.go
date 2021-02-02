// +build linux

package analyze

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

// GetDevicesInfo returns usage info about mounted devices (by calling Statfs syscall)
func (t LinuxDevicesInfoGetter) GetDevicesInfo() ([]*Device, error) {
	file, err := os.Open(t.MountsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return processMounts(file)
}

func processMounts(file io.Reader) ([]*Device, error) {
	devices := []*Device{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "/snap/") {
			continue
		}

		if strings.HasPrefix(line, "/dev") || strings.Contains(line, " zfs ") {
			parts := strings.Fields(line)
			info := &syscall.Statfs_t{}
			syscall.Statfs(parts[1], info)

			device := &Device{
				Name:       parts[0],
				MountPoint: parts[1],
				Size:       int64(info.Bsize) * int64(info.Blocks),
				Free:       int64(info.Bsize) * int64(info.Bavail),
			}
			devices = append(devices, device)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}
