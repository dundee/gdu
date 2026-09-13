package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNestedMountpointsThroughDataVolume(t *testing.T) {
	mounts := Devices{
		{MountPoint: "/"},
		{MountPoint: "/System/Volumes/Data"},
		{MountPoint: "/System/Volumes/Data/home"},
		{MountPoint: "/Volumes/USB"},
	}

	assert.Equal(t,
		[]string{"/System/Volumes/Data/home", "/System/Volumes/Data/Volumes/USB"},
		GetNestedMountpointsPaths("/System/Volumes/Data", mounts),
	)
	assert.Equal(t,
		[]string{"/System/Volumes/Data/Volumes/USB"},
		GetNestedMountpointsPaths("/System/Volumes/Data/Volumes", mounts),
	)
	assert.Equal(t,
		[]string{"/System/Volumes/Data", "/System/Volumes/Data/home", "/Volumes/USB"},
		GetNestedMountpointsPaths("/", mounts),
	)
	assert.Equal(t, []string{"/Volumes/USB"}, GetNestedMountpointsPaths("/Volumes", mounts))
}
