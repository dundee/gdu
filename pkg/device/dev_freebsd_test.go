// +build freebsd

package device

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDevicesInfo(t *testing.T) {
	getter := FreeBSDDevicesInfoGetter{MountCmd: "/sbin/mount"}
	devices, _ := getter.GetDevicesInfo()
	assert.IsType(t, Devices{}, devices)
}

func TestGetDevicesInfoFail(t *testing.T) {
	getter := FreeBSDDevicesInfoGetter{MountCmd: "/nonexistent"}
	_, err := getter.GetDevicesInfo()
	assert.Equal(t, "fork/exec /nonexistent: no such file or directory", err.Error())
}

func TestZfsMountsShown(t *testing.T) {
	mounts, _ := readMountOutput(strings.NewReader(`/dev/ada0p2 on / (ufs, local, soft-updates)
devfs on /dev (devfs)
tmpfs on /tmp (tmpfs, local)
fdescfs on /dev/fd (fdescfs)
procfs on /proc (procfs, local)
t on /t (zfs, local, nfsv4acls)
t/db on /t/db (zfs, local, nfsv4acls)
t/vm on /t/vm (zfs, local, nfsv4acls)
t/log/pflog on /var/log/pflog (zfs, local, nfsv4acls)
t/log on /t/log (zfs, local, nfsv4acls)
devfs on /compat/linux/dev (devfs)
fdescfs on /compat/linux/dev/fd (fdescfs)
tmpfs on /compat/linux/dev/shm (tmpfs, local)
map -hosts on /net (autofs)
argon:/usr/src on /usr/src (nfs)
argon:/usr/obj on /usr/obj (nfs)`))

	devices, err := processMounts(mounts, true)
	assert.Len(t, devices, 6)
	assert.Nil(t, err)
}
