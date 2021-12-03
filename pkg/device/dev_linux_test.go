//go:build linux
// +build linux

package device

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDevicesInfo(t *testing.T) {
	getter := LinuxDevicesInfoGetter{MountsPath: "/proc/mounts"}
	devices, _ := getter.GetDevicesInfo()
	assert.IsType(t, Devices{}, devices)
}

func TestGetDevicesInfoFail(t *testing.T) {
	getter := LinuxDevicesInfoGetter{MountsPath: "/xxxyyy"}
	_, err := getter.GetDevicesInfo()
	assert.Equal(t, "open /xxxyyy: no such file or directory", err.Error())
}

func TestSnapMountsNotShown(t *testing.T) {
	mounts, _ := readMountsFile(strings.NewReader(`/dev/loop4 /var/lib/snapd/snap/core18/1944 squashfs ro,nodev,relatime 0 0
/dev/loop3 /var/lib/snapd/snap/core20/904 squashfs ro,nodev,relatime 0 0
/dev/nvme0n1p1 /boot vfat rw,relatime,fmask=0022,dmask=0022,codepage=437,iocharset=ascii,shortname=mixed,utf8,errors=remount-ro 0 0`))

	devices, err := processMounts(mounts, true)
	assert.Len(t, devices, 1)
	assert.Nil(t, err)
}

func TestZfsMountsShown(t *testing.T) {
	mounts, _ := readMountsFile(strings.NewReader(`rootpool/opt /opt zfs rw,nodev,relatime,xattr,posixacl 0 0
rootpool/usr/local /usr/local zfs rw,nodev,relatime,xattr,posixacl 0 0
rootpool/home/root /root zfs rw,nodev,relatime,xattr,posixacl 0 0
rootpool/usr/games /usr/games zfs rw,nodev,relatime,xattr,posixacl 0 0
rootpool/home /home zfs rw,nodev,relatime,xattr,posixacl 0 0
/dev/loop4 /var/lib/snapd/snap/core18/1944 squashfs ro,nodev,relatime 0 0
/dev/loop3 /var/lib/snapd/snap/core20/904 squashfs ro,nodev,relatime 0 0
/dev/nvme0n1p1 /boot vfat rw,relatime,fmask=0022,dmask=0022,codepage=437,iocharset=ascii,shortname=mixed,utf8,errors=remount-ro 0 0`))

	devices, err := processMounts(mounts, true)
	assert.Len(t, devices, 6)
	assert.Nil(t, err)
}

func TestNfsMountsShown(t *testing.T) {
	mounts, _ := readMountsFile(strings.NewReader(`host1:/dir1/ /mnt/dir1 nfs4 rw,nosuid,nodev,noatime,nodiratime,vers=4.2,rsize=1048576,wsize=1048576,namlen=255,hard,proto=tcp,timeo=600,retrans=2,sec=sys,clientaddr=192.168.1.1,fsc,local_lock=none,addr=192.168.1.2 0 0
host2:/dir2/ /mnt/dir2 nfs rw,relatime,vers=3,rsize=524288,wsize=524288,namlen=255,hard,proto=tcp,timeo=600,retrans=2,sec=sys,mountaddr=192.168.1.3,mountvers=3,mountport=38081,mountproto=udp,fsc,local_lock=none,addr=192.168.1.4 0 0`))

	devices, err := processMounts(mounts, true)
	assert.Len(t, devices, 2)
	assert.Equal(t, "host1:/dir1/", devices[0].Name)
	assert.Equal(t, "/mnt/dir1", devices[0].MountPoint)
	assert.Nil(t, err)
}

func TestMountsWithSpaces(t *testing.T) {
	mounts, _ := readMountsFile(strings.NewReader(`host1:/dir1/ /mnt/dir\040with\040spaces nfs4 rw,nosuid,nodev,noatime,nodiratime,vers=4.2,rsize=1048576,wsize=1048576,namlen=255,hard,proto=tcp,timeo=600,retrans=2,sec=sys,clientaddr=192.168.1.1,fsc,local_lock=none,addr=192.168.1.2 0 0
host2:/dir2/ /mnt/dir2 nfs rw,relatime,vers=3,rsize=524288,wsize=524288,namlen=255,hard,proto=tcp,timeo=600,retrans=2,sec=sys,mountaddr=192.168.1.3,mountvers=3,mountport=38081,mountproto=udp,fsc,local_lock=none,addr=192.168.1.4 0 0`))

	devices, err := processMounts(mounts, true)
	assert.Len(t, devices, 2)
	assert.Equal(t, "host1:/dir1/", devices[0].Name)
	assert.Equal(t, "/mnt/dir with spaces", devices[0].MountPoint)
	assert.Nil(t, err)
}
