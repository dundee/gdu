//go:build openbsd
// +build openbsd

package device

import (
	"fmt"
	"strings"

	"golang.org/x/sys/unix"
)

func processMounts(mounts Devices, ignoreErrors bool) (Devices, error) {
	devices := Devices{}

	for _, mount := range mounts {
		if strings.HasPrefix(mount.Name, "/dev") || mount.Fstype == "zfs" {
			info := &unix.Statfs_t{}
			err := unix.Statfs(mount.MountPoint, info)
			if err != nil && !ignoreErrors {
				return nil, fmt.Errorf("getting stats for mount point: \"%s\", %w", mount.MountPoint, err)
			}

			mount.Size = int64(info.F_bsize) * int64(info.F_blocks)
			mount.Free = int64(info.F_bsize) * int64(info.F_bavail)

			devices = append(devices, mount)
		}
	}

	return devices, nil
}
