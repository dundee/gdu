package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNested(t *testing.T) {
	item := &Device{
		MountPoint: "/xxx",
	}
	nested := &Device{
		MountPoint: "/xxx/yyy",
	}
	notNested := &Device{
		MountPoint: "/zzz/yyy",
	}

	mounts := Devices{item, nested, notNested}

	mountsNested := GetNestedMountpointsPaths("/xxx", mounts)

	assert.Len(t, mountsNested, 1)
	assert.Equal(t, "/xxx/yyy", mountsNested[0])
}
