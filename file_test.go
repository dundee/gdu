package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFind(t *testing.T) {
	dir := File{
		name:      "xxx",
		size:      5,
		itemCount: 2,
	}

	file := &File{
		name:      "yyy",
		size:      2,
		itemCount: 1,
		parent:    &dir,
	}
	file2 := &File{
		name:      "zzz",
		size:      3,
		itemCount: 1,
		parent:    &dir,
	}
	dir.files = []*File{file, file2}

	assert.Equal(t, 0, dir.files.Find(file))
	assert.Equal(t, 1, dir.files.Find(file2))
}

func TestRemove(t *testing.T) {
	dir := File{
		name:      "xxx",
		size:      5,
		itemCount: 2,
	}

	file := &File{
		name:      "yyy",
		size:      2,
		itemCount: 1,
		parent:    &dir,
	}
	file2 := &File{
		name:      "zzz",
		size:      3,
		itemCount: 1,
		parent:    &dir,
	}
	dir.files = []*File{file, file2}

	dir.files = dir.files.Remove(file)

	assert.Equal(t, 1, len(dir.files))
	assert.Equal(t, file2, dir.files[0])
}

func TestRemoveFile(t *testing.T) {
	dir := &File{
		name:      "xxx",
		size:      5,
		itemCount: 3,
	}

	subdir := &File{
		name:      "yyy",
		size:      4,
		itemCount: 2,
		parent:    dir,
	}
	file := &File{
		name:      "zzz",
		size:      3,
		itemCount: 1,
		parent:    subdir,
	}
	dir.files = []*File{subdir}
	subdir.files = []*File{file}

	subdir.RemoveFile(file)

	assert.Equal(t, 0, len(subdir.files))
	assert.Equal(t, 1, subdir.itemCount)
	assert.Equal(t, int64(1), subdir.size)
	assert.Equal(t, 1, len(dir.files))
	assert.Equal(t, 2, dir.itemCount)
	assert.Equal(t, int64(2), dir.size)
}
