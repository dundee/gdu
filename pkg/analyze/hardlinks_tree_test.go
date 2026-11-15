package analyze

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// devicesTree is /top on device 1 holding a/f and b/g on device 1, and m/h on
// device 2, all three with inode 42.
func devicesTree() *Dir {
	top := &Dir{File: &File{Name: "top", Flag: ' '}, BasePath: "/", Dev: 1}
	for _, d := range []struct {
		dir, file string
		dev       uint64
	}{{"a", "f", 1}, {"b", "g", 1}, {"m", "h", 2}} {
		dir := &Dir{File: &File{Name: d.dir, Flag: ' ', Parent: top}, Dev: d.dev}
		dir.Files = fs.Files{&File{Name: d.file, Flag: 'H', Size: 100, Usage: 4096, Mli: 42, Parent: dir}}
		top.Files = append(top.Files, dir)
	}
	return top
}

func entryNamed(t *testing.T, dir *Dir, name string) fs.Item {
	t.Helper()
	idx, ok := dir.Files.FindByName(name)
	require.True(t, ok, "no %s in %s", name, dir.GetPath())
	return dir.Files[idx]
}

// Inode numbers repeat across filesystems: files on two devices sharing one
// are two files.
func TestHardLinksOnTwoDevicesAreTwoFiles(t *testing.T) {
	top := devicesTree()

	links := top.UpdateStats(false)

	assert.Equal(t, int64(2*4096), top.GetUsage())
	assert.Equal(t, int64(2*100), top.GetSize())
	assert.Equal(t, int64(7), top.GetItemCount())

	f := entryNamed(t, entryNamed(t, top, "a").(*Dir), "f")
	h := entryNamed(t, entryNamed(t, top, "m").(*Dir), "h")
	key, ok := HardLinkKey(f)
	require.True(t, ok)
	assert.Len(t, links.Links(key), 2, "f and g")
	key, ok = HardLinkKey(h)
	require.True(t, ok)
	assert.Len(t, links.Links(key), 1, "h alone")
}

// Each computation of the totals has an index of its own, so computing them
// again gives the same totals.
func TestTotalsComputedAgainAreTheSame(t *testing.T) {
	top := devicesTree()
	top.UpdateStats(false)
	usage, size := top.GetUsage(), top.GetSize()

	top.UpdateStats(false)

	assert.Equal(t, usage, top.GetUsage())
	assert.Equal(t, size, top.GetSize())
}

// Removing a directory holding the counted link of a file linked from
// elsewhere leaves the file counted through the other link.
func TestRemovingDirectoryWithCountedHardLink(t *testing.T) {
	top := devicesTree()
	top.UpdateStats(false)
	a, b := entryNamed(t, top, "a").(*Dir), entryNamed(t, top, "b").(*Dir)
	require.Equal(t, int64(4096), a.GetUsage(), "f is counted, as the first met")
	usage := top.GetUsage()

	top.RemoveFile(a)

	assert.Equal(t, usage, top.GetUsage())
	assert.Equal(t, int64(4096), b.GetUsage())
	assert.Equal(t, int64(5), top.GetItemCount())
}

// The device of a directory on another filesystem than its parent's is
// exported, so an imported analysis tells hard links on the two apart.
func TestDevicesSurviveExport(t *testing.T) {
	top := devicesTree()
	top.UpdateStats(false)
	var out bytes.Buffer
	require.NoError(t, top.EncodeJSON(&out, true, nil))
	assert.Contains(t, out.String(), `"name":"m","asize":100,"dsize":4096,"items":2,"dev":2`)
	assert.NotContains(t, out.String(), `"dev":1`, "the device is written only where it changes")

	imported, err := ReadAnalysis(bytes.NewBufferString(`[1,2,{"progname":"gdu"},` + out.String() + `]`))
	require.NoError(t, err)
	imported.UpdateStats(false)

	assert.Equal(t, uint64(0), imported.Dev)
	assert.Equal(t, uint64(0), entryNamed(t, imported, "a").(*Dir).Dev)
	assert.Equal(t, uint64(2), entryNamed(t, imported, "m").(*Dir).Dev)
	assert.Equal(t, int64(2*4096), imported.GetUsage())
}
