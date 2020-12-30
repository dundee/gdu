package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFind(t *testing.T) {
	dir := File{
		Name:      "xxx",
		Size:      5,
		ItemCount: 2,
	}

	file := &File{
		Name:      "yyy",
		Size:      2,
		ItemCount: 1,
		Parent:    &dir,
	}
	file2 := &File{
		Name:      "zzz",
		Size:      3,
		ItemCount: 1,
		Parent:    &dir,
	}
	dir.Files = []*File{file, file2}

	assert.Equal(t, 0, dir.Files.Find(file))
	assert.Equal(t, 1, dir.Files.Find(file2))
}

func TestRemove(t *testing.T) {
	dir := File{
		Name:      "xxx",
		Size:      5,
		ItemCount: 2,
	}

	file := &File{
		Name:      "yyy",
		Size:      2,
		ItemCount: 1,
		Parent:    &dir,
	}
	file2 := &File{
		Name:      "zzz",
		Size:      3,
		ItemCount: 1,
		Parent:    &dir,
	}
	dir.Files = []*File{file, file2}

	dir.Files = dir.Files.Remove(file)

	assert.Equal(t, 1, len(dir.Files))
	assert.Equal(t, file2, dir.Files[0])
}

func TestRemoveFile(t *testing.T) {
	dir := &File{
		Name:      "xxx",
		Size:      5,
		ItemCount: 3,
	}

	subdir := &File{
		Name:      "yyy",
		Size:      4,
		ItemCount: 2,
		Parent:    dir,
	}
	file := &File{
		Name:      "zzz",
		Size:      3,
		ItemCount: 1,
		Parent:    subdir,
	}
	dir.Files = []*File{subdir}
	subdir.Files = []*File{file}

	subdir.RemoveFile(file)

	assert.Equal(t, 0, len(subdir.Files))
	assert.Equal(t, 1, subdir.ItemCount)
	assert.Equal(t, int64(1), subdir.Size)
	assert.Equal(t, 1, len(dir.Files))
	assert.Equal(t, 2, dir.ItemCount)
	assert.Equal(t, int64(2), dir.Size)
}
