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

	assert.Equal(t, int64(5), dir.Size)
	assert.Equal(t, 42, dir.GetMtime().Minute())
}

func TestUpdateStatsWithFileFiltering(t *testing.T) {
	dir := Dir{
		File: &File{
			Name:  "xxx",
			Size:  1,
			Mtime: time.Date(2021, 8, 19, 0, 40, 0, 0, time.UTC),
		},
		ItemCount: 1,
	}

	file := &File{
		Name:   "yyy.go",
		Size:   2,
		Usage:  2048,
		Mtime:  time.Date(2021, 8, 19, 0, 41, 0, 0, time.UTC),
		Parent: &dir,
	}
	file2 := &File{
		Name:   "zzz.go",
		Size:   3,
		Usage:  1024,
		Mtime:  time.Date(2021, 8, 19, 0, 42, 0, 0, time.UTC),
		Parent: &dir,
	}
	subdir := &Dir{
		File: &File{
			Name:   "subdir",
			Size:   4,
			Mtime:  time.Date(2021, 8, 19, 0, 43, 0, 0, time.UTC),
			Parent: &dir,
		},
		ItemCount: 1,
	}
	subsubdir := &Dir{
		File: &File{
			Name:   "subsubdir",
			Size:   4,
			Mtime:  time.Date(2021, 8, 19, 0, 43, 0, 0, time.UTC),
			Parent: subdir,
		},
		ItemCount: 1,
	}
	subdir.Files = fs.Files{subsubdir}
	dir.Files = fs.Files{file, file2, subdir}

	dir.UpdateStatsWithFileFiltering(nil)

	assert.Equal(t, int64(1024+5), dir.Size)
	assert.Equal(t, int64(4), dir.ItemCount)
	assert.Equal(t, int64(1024+2048), dir.Usage)
	assert.Equal(t, 43, dir.GetMtime().Minute())
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

func TestStatsFromJSONArePreserved(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name:  "summary",
			Size:  4096,
			Usage: 2048,
		},
		ItemCount: 2,
	}
	dir.SetStatsFromJSON()

	dir.UpdateStats(make(fs.HardLinkedItems, 10))
	assert.Equal(t, int64(4096), dir.GetSize())
	assert.Equal(t, int64(2048), dir.GetUsage())
	assert.Equal(t, int64(2), dir.GetItemCount())

	dir.UpdateStatsWithFileFiltering(make(fs.HardLinkedItems, 10))
	count, size, usage := dir.GetItemStats(make(fs.HardLinkedItems, 10), true)
	assert.Equal(t, int64(2), count)
	assert.Equal(t, int64(4096), size)
	assert.Equal(t, int64(2048), usage)
}
