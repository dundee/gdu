package testanalyze

import (
	"errors"

	"github.com/dundee/gdu/v4/analyze"
)

// MockedAnalyzer returns dir with files with diferent size exponents
type MockedAnalyzer struct{}

// AnalyzeDir returns dir with files with diferent size exponents
func (a *MockedAnalyzer) AnalyzeDir(path string, ignore analyze.ShouldDirBeIgnored) *analyze.Dir {
	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "test_dir",
			Usage: 1e12 + 1,
			Size:  1e12 + 2,
		},
		BasePath:  ".",
		ItemCount: 12,
	}
	file := &analyze.Dir{
		File: &analyze.File{
			Name:   "aaa",
			Usage:  1e12 + 1,
			Size:   1e12 + 2,
			Parent: dir,
		},
		ItemCount: 5,
	}
	file2 := &analyze.Dir{
		File: &analyze.File{
			Name:   "bbb",
			Usage:  1e9 + 1,
			Size:   1e9 + 2,
			Parent: dir,
		},
		ItemCount: 3,
	}
	file3 := &analyze.Dir{
		File: &analyze.File{
			Name:   "ccc",
			Usage:  1e6 + 1,
			Size:   1e6 + 2,
			Parent: dir,
		},
		ItemCount: 2,
	}
	file4 := &analyze.File{
		Name:   "ddd",
		Usage:  1e3 + 1,
		Size:   1e3 + 2,
		Parent: dir,
	}
	dir.Files = analyze.Files{file, file2, file3, file4}

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
