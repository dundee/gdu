//go:build netbsd
// +build netbsd

package device

import (
	"strings"

	"golang.org/x/sys/unix"
)

func processMounts(mounts Devices, ignoreErrors bool) (Devices, error) {
	devices := Devices{}

	for _, mount := range mounts {
		if strings.HasPrefix(mount.Name, "/dev") || mount.Fstype == "zfs" {
			info := &unix.Statvfs_t{}
			err := unix.Statvfs(mount.MountPoint, info)
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
