package analyze

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
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
	dir.Files = Files{file}

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
	dir.Files = Files{file, file2}

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
	dir.Files = Files{file, file2}

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
	dir.Files = Files{file, file2}

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
	dir.Files = Files{file, file2}

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
	dir.Files = Files{file}

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
	dir.Files = Files{file}

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
	dir.Files = Files{subdir}
	subdir.Files = Files{file}

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

func TestRemoveFileWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Chmod("test_dir/nested", 0)
	assert.Nil(t, err)
	defer func() {
		err = os.Chmod("test_dir/nested", 0755)
		assert.Nil(t, err)
	}()

	dir := &Dir{
		File: &File{
			Name: "test_dir",
		},
		BasePath: ".",
	}

	subdir := &Dir{
		File: &File{
			Name:   "nested",
			Parent: dir,
		},
	}

	err = RemoveItemFromDir(dir, subdir)
	assert.Contains(t, err.Error(), "permission denied")
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
	dir.Files = Files{subdir}
	subdir.Files = Files{file}

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
	dir.Files = Files{subdir}
	subdir.Files = Files{file}

	err := EmptyFileFromDir(subdir, file)

	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestUpdateStats(t *testing.T) {
	dir := Dir{
		File: &File{
			Name: "xxx",
			Size: 1,
		},
		ItemCount: 1,
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
	dir.Files = Files{file, file2}

	dir.UpdateStats(nil)

	assert.Equal(t, int64(4096+5), dir.Size)
}
