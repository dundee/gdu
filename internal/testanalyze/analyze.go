package testanalyze

import (
	"errors"

	"github.com/dundee/gdu/analyze"
)

// MockedProcessDir returns dir with files with diferent size exponents
func MockedProcessDir(path string, progress *analyze.CurrentProgress, ignore analyze.ShouldDirBeIgnored) *analyze.File {
	dir := &analyze.File{
		Name:      "test_dir",
		BasePath:  ".",
		Usage:     1e12 + 1,
		Size:      1e12 + 2,
		ItemCount: 12,
	}
	file := &analyze.File{
		Name:      "aaa",
		Usage:     1e12 + 1,
		Size:      1e12 + 2,
		Parent:    dir,
		ItemCount: 5,
	}
	file2 := &analyze.File{
		Name:      "bbb",
		Usage:     1e9 + 1,
		Size:      1e9 + 2,
		Parent:    dir,
		ItemCount: 3,
	}
	file3 := &analyze.File{
		Name:      "ccc",
		Usage:     1e6 + 1,
		Size:      1e6 + 2,
		Parent:    dir,
		ItemCount: 2,
	}
	file4 := &analyze.File{
		Name:      "ddd",
		Usage:     1e3 + 1,
		Size:      1e3 + 2,
		Parent:    dir,
		ItemCount: 1,
	}
	dir.Files = analyze.Files{file, file2, file3, file4}

	progress.Mutex.Lock()
	progress.Done = true
	progress.Mutex.Unlock()

	return dir
}

// RemoveFileFromDirWithErr returns error
func RemoveFileFromDirWithErr(dir *analyze.File, file *analyze.File) error {
	return errors.New("Failed")
}
