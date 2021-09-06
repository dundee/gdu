package device

import (
	"sort"
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

func TestSortByName(t *testing.T) {
	item := &Device{
		Name: "/xxx",
	}
	nested := &Device{
		Name: "/xxx/yyy",
	}
	notNested := &Device{
		Name: "/zzz/yyy",
	}

	devices := Devices{item, nested, notNested}

	sort.Sort(ByName(devices))

	assert.Equal(t, "/zzz/yyy", devices[0].Name)
	assert.Equal(t, "/xxx/yyy", devices[1].Name)
	assert.Equal(t, "/xxx", devices[2].Name)
}

func TestSortByUsedSize(t *testing.T) {
	item := &Device{
		Name: "xxx",
		Size: 1e12,
		Free: 1e3,
	}
	nested := &Device{
		Name: "yyy",
		Size: 1e12,
		Free: 1e6,
	}
	notNested := &Device{
		Name: "zzz",
		Size: 1e12,
		Free: 1e12,
	}

	devices := Devices{item, nested, notNested}

	sort.Sort(sort.Reverse(ByUsedSize(devices)))

	assert.Equal(t, "zzz", devices[0].Name)
	assert.Equal(t, "yyy", devices[1].Name)
	assert.Equal(t, "xxx", devices[2].Name)
}
