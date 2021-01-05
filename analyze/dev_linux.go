// +build linux,amd64

package analyze

import (
	"bufio"
	"io"
	"os"
	"strings"
	"syscall"
)

// GetDevicesInfo returns usage info about mounted devices (by calling Statfs syscall)
func GetDevicesInfo(mountsPath string) ([]*Device, error) {
	file, err := os.Open(mountsPath)
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

		if line[0:4] == "/dev" || strings.Contains(line, " zfs ") {
			parts := strings.Fields(line)
			info := &syscall.Statfs_t{}
			syscall.Statfs(parts[1], info)

			device := &Device{
				Name:       parts[0],
				MountPoint: parts[1],
				Size:       info.Bsize * int64(info.Blocks),
				Free:       info.Bsize * int64(info.Bavail),
			}
			devices = append(devices, device)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}
