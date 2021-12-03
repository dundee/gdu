//go:build darwin
// +build darwin

package device

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDevicesInfo(t *testing.T) {
	getter := DarwinDevicesInfoGetter{MountCmd: "/sbin/mount"}
	devices, _ := getter.GetDevicesInfo()
	assert.IsType(t, Devices{}, devices)
}

func TestGetDevicesInfoFail(t *testing.T) {
	getter := DarwinDevicesInfoGetter{MountCmd: "/nonexistent"}
	_, err := getter.GetDevicesInfo()
	assert.Equal(t, "fork/exec /nonexistent: no such file or directory", err.Error())
}

func TestMountsWithSpace(t *testing.T) {
	mounts, err := readMountOutput(strings.NewReader(`//inglor@vault.lan/volatile on /Users/inglor/Mountpoints/volatile (vault.lan) (smbfs, nodev, nosuid, mounted by inglor)`))
	assert.Equal(t, "//inglor@vault.lan/volatile", mounts[0].Name)
	assert.Equal(t, "/Users/inglor/Mountpoints/volatile (vault.lan)", mounts[0].MountPoint)
	assert.Equal(t, "smbfs", mounts[0].Fstype)
	assert.Nil(t, err)
}
