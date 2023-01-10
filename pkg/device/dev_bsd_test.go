//go:build freebsd || openbsd || netbsd || darwin
// +build freebsd openbsd netbsd darwin

package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDevicesInfo(t *testing.T) {
	getter := BSDDevicesInfoGetter{MountCmd: "/sbin/mount"}
	devices, _ := getter.GetDevicesInfo()
	assert.IsType(t, Devices{}, devices)
}

func TestGetDevicesInfoFail(t *testing.T) {
	getter := BSDDevicesInfoGetter{MountCmd: "/nonexistent"}
	_, err := getter.GetDevicesInfo()
	assert.Equal(t, "fork/exec /nonexistent: no such file or directory", err.Error())
}
