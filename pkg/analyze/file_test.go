package analyze

import (
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestIsDir(t *testing.T) {
	dir := Dir{
		File: &File{
			Name: "xxx",
			Size: 5,
		},
		ItemCount: 2,
	}
	file := &File{
		Name:   "yyy",
		Size:   2,
		Parent: &dir,
	}
	dir.Files = fs.Files{file}

	assert.True(t, dir.IsDir())
	assert.False(t, file.IsDir())
}

func TestGetType(t *testing.T) {
	dir := Dir{
		File: &File{
			Name: "xxx",
			Size: 5,
		},
		ItemCount: 2,
	}
	file := &File{
		Name:   "yyy",
		Size:   2,
		Parent: &dir,
		Flag:   ' ',
	}
	file2 := &File{
		Name:   "yyy",
		Size:   2,
		Parent: &dir,
		Flag:   '@',
	}
	dir.Files = fs.Files{file, file2}

	assert.Equal(t, "Directory", dir.GetType())
	assert.Equal(t, "File", file.GetType())
	assert.Equal(t, "Other", file2.GetType())
}

func TestFind(t *testing.T) {
	dir := Dir{
		File: &File{
			Name: "xxx",
			Size: 5,
		},
		ItemCount: 2,
	}

	file := &File{
		Name:   "yyy",
		Size:   2,
		Parent: &dir,
	}
	file2 := &File{
		Name:   "zzz",
		Size:   3,
		Parent: &dir,
	}
	dir.Files = fs.Files{file, file2}

	i, _ := dir.Files.IndexOf(file)
	assert.Equal(t, 0, i)
	i, _ = dir.Files.IndexOf(file2)
	assert.Equal(t, 1, i)
}

func TestRemove(t *testing.T) {
	dir := Dir{
		File: &File{
			Name: "xxx",
			Size: 5,
		},
		ItemCount: 2,
	}

	file := &File{
		Name:   "yyy",
		Size:   2,
		Parent: &dir,
	}
	file2 := &File{
		Name:   "zzz",
		Size:   3,
		Parent: &dir,
	}
	dir.Files = fs.Files{file, file2}

	dir.Files = dir.Files.Remove(file)

	assert.Equal(t, 1, len(dir.Files))
	assert.Equal(t, file2, dir.Files[0])
}

func TestRemoveByName(t *testing.T) {
	dir := Dir{
		File: &File{
			Name:  "xxx",
			Size:  5,
			Usage: 8,
		},
		ItemCount: 2,
	}

	file := &File{
		Name:   "yyy",
		Size:   2,
		Usage:  4,
		Parent: &dir,
	}
	file2 := &File{
		Name:   "zzz",
		Size:   3,
		Usage:  4,
		Parent: &dir,
	}
	dir.Files = fs.Files{file, file2}

	dir.Files = dir.Files.RemoveByName("yyy")

	assert.Equal(t, 1, len(dir.Files))
	assert.Equal(t, file2, dir.Files[0])
}

func TestRemoveNotInDir(t *testing.T) {
	dir := Dir{
		File: &File{
			Name:  "xxx",
			Size:  5,
			Usage: 8,
		},
		ItemCount: 2,
	}

	file := &File{
		Name:   "yyy",
		Size:   2,
		Usage:  4,
		Parent: &dir,
	}
	file2 := &File{
		Name:  "zzz",
		Size:  3,
		Usage: 4,
	}
	dir.Files = fs.Files{file}

	_, ok := dir.Files.IndexOf(file2)
	assert.Equal(t, false, ok)

	dir.Files = dir.Files.Remove(file2)

	assert.Equal(t, 1, len(dir.Files))
}

func TestRemoveByNameNotInDir(t *testing.T) {
	dir := Dir{
		File: &File{
			Name:  "xxx",
			Size:  5,
			Usage: 8,
		},
		ItemCount: 2,
	}

	file := &File{
		Name:   "yyy",
		Size:   2,
		Usage:  4,
		Parent: &dir,
	}
	file2 := &File{
		Name:  "zzz",
		Size:  3,
		Usage: 4,
	}
	dir.Files = fs.Files{file}

	_, ok := dir.Files.IndexOf(file2)
	assert.Equal(t, false, ok)

	dir.Files = dir.Files.RemoveByName("zzz")

	assert.Equal(t, 1, len(dir.Files))
}

func TestRemoveFile(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name:  "xxx",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  ".",
	}

	subdir := &Dir{
		File: &File{
			Name:   "yyy",
			Size:   4,
			Usage:  8,
			Parent: dir,
		},
		ItemCount: 2,
	}
	file := &File{
		Name:   "zzz",
		Size:   3,
		Usage:  4,
		Parent: subdir,
	}
	dir.Files = fs.Files{subdir}
	subdir.Files = fs.Files{file}

	err := RemoveItemFromDir(subdir, file)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(subdir.Files))
	assert.Equal(t, 1, subdir.ItemCount)
	assert.Equal(t, int64(1), subdir.Size)
	assert.Equal(t, int64(4), subdir.Usage)
	assert.Equal(t, 1, len(dir.Files))
	assert.Equal(t, 2, dir.ItemCount)
	assert.Equal(t, int64(2), dir.Size)
}

func TestTruncateFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dir := &Dir{
		File: &File{
			Name:  "test_dir",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  ".",
	}

	subdir := &Dir{
		File: &File{
			Name:   "nested",
			Size:   4,
			Usage:  8,
			Parent: dir,
		},
		ItemCount: 2,
	}
	file := &File{
		Name:   "file2",
		Size:   3,
		Usage:  4,
		Parent: subdir,
	}
	dir.Files = fs.Files{subdir}
	subdir.Files = fs.Files{file}

	err := EmptyFileFromDir(subdir, file)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(subdir.Files))
	assert.Equal(t, 2, subdir.ItemCount)
	assert.Equal(t, int64(1), subdir.Size)
	assert.Equal(t, int64(4), subdir.Usage)
	assert.Equal(t, 1, len(dir.Files))
	assert.Equal(t, 3, dir.ItemCount)
	assert.Equal(t, int64(2), dir.Size)
}

func TestTruncateFileWithErr(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name:  "xxx",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  ".",
	}

	subdir := &Dir{
		File: &File{
			Name:   "yyy",
			Size:   4,
			Usage:  8,
			Parent: dir,
		},
		ItemCount: 2,
	}
	file := &File{
		Name:   "zzz",
		Size:   3,
		Usage:  4,
		Parent: subdir,
	}
	dir.Files = fs.Files{subdir}
	subdir.Files = fs.Files{file}

	err := EmptyFileFromDir(subdir, file)

	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestUpdateStats(t *testing.T) {
	dir := Dir{
		File: &File{
			Name:  "xxx",
			Size:  1,
			Mtime: time.Date(2021, 8, 19, 0, 40, 0, 0, time.UTC),
		},
		ItemCount: 1,
	}

	file := &File{
		Name:   "yyy",
		Size:   2,
		Mtime:  time.Date(2021, 8, 19, 0, 41, 0, 0, time.UTC),
		Parent: &dir,
	}
	file2 := &File{
		Name:   "zzz",
		Size:   3,
		Mtime:  time.Date(2021, 8, 19, 0, 42, 0, 0, time.UTC),
		Parent: &dir,
	}
	dir.Files = fs.Files{file, file2}

	dir.UpdateStats(nil)

	assert.Equal(t, int64(4096+5), dir.Size)
	assert.Equal(t, 42, dir.GetMtime().Minute())
}

func TestGetMultiLinkedInode(t *testing.T) {
	file := &File{
		Name: "xxx",
		Mli:  5,
	}

	assert.Equal(t, uint64(5), file.GetMultiLinkedInode())
}

func TestGetPathWithoutLeadingSlash(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name:  "C:\\",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  "",
	}

	assert.Equal(t, "C:\\", dir.GetPath())
}

func TestSetParent(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name:  "root",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  "/",
	}
	file := &File{
		Name: "xxx",
		Mli:  5,
	}
	file.SetParent(dir)

	assert.Equal(t, "root", file.GetParent().GetName())
}

func TestGetFiles(t *testing.T) {
	file := &File{
		Name: "xxx",
		Mli:  5,
	}
	dir := &Dir{
		File: &File{
			Name:  "root",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  "/",
		Files:     fs.Files{file},
	}

	assert.Equal(t, file.Name, dir.GetFiles()[0].GetName())
	assert.Equal(t, fs.Files{}, file.GetFiles())
}

func TestSetFilesPanicsOnFile(t *testing.T) {
	file := &File{
		Name: "xxx",
		Mli:  5,
	}
	assert.Panics(t, func() {
		file.SetFiles(fs.Files{file})
	})
}

func TestAddFilePanicsOnFile(t *testing.T) {
	file := &File{
		Name: "xxx",
		Mli:  5,
	}
	assert.Panics(t, func() {
		file.AddFile(file)
	})
}
