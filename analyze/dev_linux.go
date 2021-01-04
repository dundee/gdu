// +build linux,amd64

package analyze

import (
	"bufio"
	"log"
	"os"
	"runtime"
	"strings"
	"syscall"
)

// GetDevicesInfo returns usage info about mounted devices (by calling Statfs syscall)
func GetDevicesInfo() []*Device {
	if runtime.GOOS != "linux" {
		panic("Listing devices is not yet supported on " + runtime.GOOS)
	}

	devices := []*Device{}

	file, err := os.Open("/proc/mounts")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "/snap/") {
			continue
		}

		if line[0:4] == "/dev" {
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
		log.Fatal(err)
	}

	return devices
}
