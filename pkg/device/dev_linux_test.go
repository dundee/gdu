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
