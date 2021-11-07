package testanalyze

import (
	"errors"
	"time"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

// MockedAnalyzer returns dir with files with different size exponents
type MockedAnalyzer struct{}

// AnalyzeDir returns dir with files with different size exponents
func (a *MockedAnalyzer) AnalyzeDir(path string, ignore analyze.ShouldDirBeIgnored) *analyze.Dir {
	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "test_dir",
			Usage: 1e12 + 1,
			Size:  1e12 + 2,
			Mtime: time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
		},
		BasePath:  ".",
		ItemCount: 12,
	}
	dir2 := &analyze.Dir{
		File: &analyze.File{
			Name:   "aaa",
			Usage:  1e12 + 1,
			Size:   1e12 + 2,
			Mtime:  time.Date(2021, 8, 27, 22, 23, 27, 0, time.UTC),
			Parent: dir,
		},
	}
	dir3 := &analyze.Dir{
		File: &analyze.File{
			Name:   "bbb",
			Usage:  1e9 + 1,
			Size:   1e9 + 2,
			Mtime:  time.Date(2021, 8, 27, 22, 23, 26, 0, time.UTC),
			Parent: dir,
		},
	}
	dir4 := &analyze.Dir{
		File: &analyze.File{
			Name:   "ccc",
			Usage:  1e6 + 1,
			Size:   1e6 + 2,
			Mtime:  time.Date(2021, 8, 27, 22, 23, 25, 0, time.UTC),
			Parent: dir,
		},
	}
	file := &analyze.File{
		Name:   "ddd",
		Usage:  1e3 + 1,
		Size:   1e3 + 2,
		Mtime:  time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
		Parent: dir,
	}
	dir.Files = analyze.Files{dir2, dir3, dir4, file}

	return dir
}

// GetProgressChan returns always Done
func (a *MockedAnalyzer) GetProgressChan() chan analyze.CurrentProgress {
	return make(chan analyze.CurrentProgress)
}

// GetDoneChan returns always Done
func (a *MockedAnalyzer) GetDoneChan() chan struct{} {
	c := make(chan struct{}, 1)
	defer func() { c <- struct{}{} }()
	return c
}

// ResetProgress does nothing
func (a *MockedAnalyzer) ResetProgress() {}

// RemoveItemFromDirWithErr returns error
func RemoveItemFromDirWithErr(dir *analyze.Dir, file analyze.Item) error {
	return errors.New("Failed")
}
