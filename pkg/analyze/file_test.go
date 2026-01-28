package analyze

import (
	"slices"
	"testing"
	"time"

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

	dirFiles := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, file.Name, dirFiles[0].GetName())
	fileFiles := slices.Collect(file.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, 0, len(fileFiles))
}

func TestGetFilesLocked(t *testing.T) {
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

	unlock := dir.RLock()
	defer unlock()
	files := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))
	locked := slices.Collect(dir.GetFilesLocked(fs.SortByName, fs.SortAsc))
	assert.Equal(t, len(files), len(locked))
	assert.Equal(t, files[0].GetName(), locked[0].GetName())
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
