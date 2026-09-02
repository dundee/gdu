package analyze

import (
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func createScannedRoot(basePath, name string, size int64) *Dir {
	return &Dir{
		File: &File{
			Name: name,
			Size: size,
		},
		BasePath:  basePath,
		ItemCount: 1,
		Files: fs.Files{
			&File{Name: "file", Size: size, Usage: size},
		},
	}
}

func TestCreateVirtualRootDirKeepsRootPaths(t *testing.T) {
	first := createScannedRoot("/home/user", "projects", 10)
	second := createScannedRoot("/var", "log", 20)

	root := CreateVirtualRootDir(first, second)

	// the roots gained a parent but must still report their real location
	assert.Equal(t, "/home/user/projects", first.GetPath())
	assert.Equal(t, "/var/log", second.GetPath())
	assert.Equal(t, root, first.GetParent())
	assert.Equal(t, root, second.GetParent())
}

func TestCreateVirtualRootDirHasNoFilesystemPath(t *testing.T) {
	root := CreateVirtualRootDir(createScannedRoot("/var", "log", 1))

	assert.Equal(t, VirtualRootName, root.GetPath())
	assert.Nil(t, root.GetParent())
	assert.True(t, root.IsDir())
}

func TestCreateVirtualRootDirAggregatesStats(t *testing.T) {
	root := CreateVirtualRootDir(
		createScannedRoot("/home/user", "projects", 10),
		createScannedRoot("/var", "log", 20),
	)

	root.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, int64(30), root.GetSize())
	assert.Equal(t, int64(30), root.GetUsage())
	// two roots with one file each, plus the roots and the virtual dir itself
	assert.Equal(t, int64(5), root.GetItemCount())
}

func TestCreateVirtualRootDirWithoutRoots(t *testing.T) {
	root := CreateVirtualRootDir()

	root.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, VirtualRootName, root.GetPath())
	assert.Equal(t, int64(1), root.GetItemCount())
}

func TestIsVirtualRootDir(t *testing.T) {
	scanned := createScannedRoot("/var", "log", 1)
	root := CreateVirtualRootDir(scanned)

	assert.True(t, IsVirtualRootDir(root))
	assert.False(t, IsVirtualRootDir(scanned))
	assert.False(t, IsVirtualRootDir(&File{Name: VirtualRootName}))
}

// A real directory that happens to be called "(multiple)" must not be mistaken
// for the virtual root, or gdu would refuse to operate on it.
func TestIsVirtualRootDirIgnoresRealDirWithSameName(t *testing.T) {
	scanned := createScannedRoot("/home/user", VirtualRootName, 1)

	assert.False(t, IsVirtualRootDir(scanned))
}

func TestVirtualRootDirSubtractsStatsOnRemove(t *testing.T) {
	first := createScannedRoot("/home/user", "projects", 10)
	second := createScannedRoot("/var", "log", 20)
	root := CreateVirtualRootDir(first, second)
	root.UpdateStats(make(fs.HardLinkedItems))

	root.RemoveFile(second)

	assert.Equal(t, int64(10), root.GetSize())
	assert.Equal(t, int64(10), root.GetUsage())
}
