package analyze

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"slices"
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
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false }, false,
	).(*StoredDir)

	a.GetDone().Wait()

	dir.UpdateStats(make(fs.HardLinkedItems))

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7+4096*3), dir.Size)
	assert.Equal(t, 5, dir.ItemCount)
	assert.True(t, dir.IsDir())

	// test dir tree
	files := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, "nested", files[0].GetName())

	nested := files[0].(*StoredDir)
	nestedFiles := slices.Collect(nested.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, "subnested", nestedFiles[1].GetName())

	// test file
	assert.Equal(t, "file2", nestedFiles[0].GetName())
	assert.Equal(t, int64(2), nestedFiles[0].GetSize())
	assert.Equal(t, int64(4096), nestedFiles[0].GetUsage())

	subnested := nestedFiles[1].(*StoredDir)
	subnestedFiles := slices.Collect(subnested.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, "file", subnestedFiles[0].GetName())
	assert.Equal(t, int64(5), subnestedFiles[0].GetSize())
}

func TestRemoveStoredFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateStoredAnalyzer("/tmp/badger")
	dir := a.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false }, false,
	).(*StoredDir)

	a.GetDone().Wait()
	a.ResetProgress()

	dir.UpdateStats(make(fs.HardLinkedItems))

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7+4096*3), dir.Size)
	assert.Equal(t, 5, dir.ItemCount)
	assert.True(t, dir.IsDir())

	dirFiles := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))
	subdir := dirFiles[0].(*StoredDir)
	subdirFiles := slices.Collect(subdir.GetFiles(fs.SortByName, fs.SortAsc))
	subdir.RemoveFile(subdirFiles[0])

	closeFn := DefaultStorage.Open()
	defer closeFn()
	stored, err := DefaultStorage.GetDirForPath("test_dir")
	assert.NoError(t, err)

	assert.Equal(t, 4, stored.GetItemCount())
	assert.Equal(t, int64(5+4096*3), stored.GetSize())

	storedFiles := slices.Collect(stored.GetFiles(fs.SortByName, fs.SortAsc))
	storedNested := storedFiles[0].(*StoredDir)
	storedNestedFiles := slices.Collect(storedNested.GetFiles(fs.SortByName, fs.SortAsc))
	storedSubnested := storedNestedFiles[0].(*StoredDir)
	storedSubnestedFiles := slices.Collect(storedSubnested.GetFiles(fs.SortByName, fs.SortAsc))
	file := storedSubnestedFiles[0]
	assert.Equal(t, false, file.IsDir())
	assert.Equal(t, "file", file.GetName())
	assert.Equal(t, "test_dir/nested/subnested", file.GetParent().GetPath())
}

func TestParentDirGetNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetName()
}

func TestParentDirGetFlagPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetFlag()
}

func TestParentDirIsDirPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.IsDir()
}

func TestParentDirGetSizePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetSize()
}

func TestParentDirGetTypePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetType()
}

func TestParentDirGetUsagePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetUsage()
}

func TestParentDirGetMtimePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetMtime()
}

func TestParentDirGetItemCountPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetItemCount()
}

func TestParentDirGetParentPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetParent()
}

func TestParentDirSetParentPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.SetParent(nil)
}

func TestParentDirGetMultiLinkedInodePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetMultiLinkedInode()
}

func TestParentDirEncodeJSONPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	err := dir.EncodeJSON(nil, false)
	assert.NoError(t, err)
}

func TestParentDirUpdateStatsPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.UpdateStats(nil)
}

func TestParentDirAddFilePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.AddFile(nil)
}

func TestParentDirGetFilesPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetFiles(fs.SortByName, fs.SortAsc)
}

func TestParentDirGetFilesLockedPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetFilesLocked(fs.SortByName, fs.SortAsc)
}

func TestParentDirRLockPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.RLock()
}

func TestParentDirRemoveFilePanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.RemoveFile(nil)
}

func TestParentDirGetItemStatsPanics(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "must not be called", r)
		}
	}()
	dir := &ParentDir{}
	dir.GetItemStats(nil)
}
