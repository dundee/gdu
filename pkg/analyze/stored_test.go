package analyze

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestEncDec(t *testing.T) {
	var d fs.Item = &StoredDir{
		Dir: &Dir{
			File: &File{
				Name: "xxx",
			},
			BasePath: "/yyy",
		},
	}

	b := &bytes.Buffer{}
	enc := gob.NewEncoder(b)
	err := enc.Encode(d)
	assert.NoError(t, err)

	var x fs.Item = &StoredDir{}
	dec := gob.NewDecoder(b)
	err = dec.Decode(x)
	assert.NoError(t, err)

	fmt.Println(d, x)
	assert.Equal(t, d.GetName(), x.GetName())
}

func TestStoredAnalyzer(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateStoredAnalyzer("/tmp/badger")
	dir := a.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*StoredDir)

	a.GetDone().Wait()

	dir.UpdateStats(make(fs.HardLinkedItems))

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7+4096*3), dir.Size)
	assert.Equal(t, 5, dir.ItemCount)
	assert.True(t, dir.IsDir())

	// test dir tree
	assert.Equal(t, "nested", dir.GetFiles()[0].GetName())
	assert.Equal(t, "subnested", dir.GetFiles()[0].(*StoredDir).GetFiles()[1].GetName())

	// test file
	assert.Equal(t, "file2", dir.GetFiles()[0].(*StoredDir).GetFiles()[0].GetName())
	assert.Equal(t, int64(2), dir.GetFiles()[0].(*StoredDir).GetFiles()[0].GetSize())
	assert.Equal(t, int64(4096), dir.GetFiles()[0].(*StoredDir).GetFiles()[0].GetUsage())

	assert.Equal(
		t, "file", dir.GetFiles()[0].(*StoredDir).GetFiles()[1].(*StoredDir).GetFiles()[0].GetName(),
	)
	assert.Equal(
		t, int64(5), dir.GetFiles()[0].(*StoredDir).GetFiles()[1].(*StoredDir).GetFiles()[0].GetSize(),
	)
}

func TestRemoveStoredFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateStoredAnalyzer("/tmp/badger")
	dir := a.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*StoredDir)

	a.GetDone().Wait()
	a.ResetProgress()

	dir.UpdateStats(make(fs.HardLinkedItems))

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7+4096*3), dir.Size)
	assert.Equal(t, 5, dir.ItemCount)
	assert.True(t, dir.IsDir())

	subdir := dir.GetFiles()[0].(*StoredDir)
	subdir.RemoveFile(subdir.GetFiles()[0])

	closeFn := DefaultStorage.Open()
	defer closeFn()
	stored, err := DefaultStorage.GetDirForPath("test_dir")
	assert.NoError(t, err)

	assert.Equal(t, 4, stored.GetItemCount())
	assert.Equal(t, int64(5+4096*3), stored.GetSize())

	file := stored.GetFiles()[0].GetFiles()[0].GetFiles()[0]
	assert.Equal(t, false, file.IsDir())
	assert.Equal(t, "file", file.GetName())
	assert.Equal(t, "test_dir/nested/subnested", file.GetParent().GetPath())
}
