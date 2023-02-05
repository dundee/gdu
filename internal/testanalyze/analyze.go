package testanalyze

import (
	"errors"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// MockedAnalyzer returns dir with files with different size exponents
type MockedAnalyzer struct{}

// AnalyzeDir returns dir with files with different size exponents
func (a *MockedAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, enableGC bool,
) fs.Item {
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
	dir.Files = fs.Files{dir2, dir3, dir4, file}

	return dir
}

// GetProgressChan returns always Done
func (a *MockedAnalyzer) GetProgressChan() chan common.CurrentProgress {
	return make(chan common.CurrentProgress)
}

// GetDone returns always Done
func (a *MockedAnalyzer) GetDone() common.SignalGroup {
	c := make(common.SignalGroup)
	defer c.Broadcast()
	return c
}

// ResetProgress does nothing
func (a *MockedAnalyzer) ResetProgress() {}

// SetFollowSymlinks does nothing
func (a *MockedAnalyzer) SetFollowSymlinks(v bool) {}

// RemoveItemFromDirWithErr returns error
func RemoveItemFromDirWithErr(dir fs.Item, file fs.Item) error {
	return errors.New("Failed")
}
