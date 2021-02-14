package testanalyze

import "github.com/dundee/gdu/analyze"

// MockedProcessDir returns dir with files with diferent size exponents
func MockedProcessDir(path string, progress *analyze.CurrentProgress, ignore analyze.ShouldDirBeIgnored) *analyze.File {
	dir := &analyze.File{
		Name:     "test_dir",
		BasePath: ".",
		Usage:    1e12 + 1,
	}
	file := &analyze.File{
		Name:   "a",
		Usage:  1e12 + 1,
		Parent: dir,
	}
	file2 := &analyze.File{
		Name:   "b",
		Usage:  1e9 + 1,
		Parent: dir,
	}
	file3 := &analyze.File{
		Name:   "c",
		Usage:  1e6 + 1,
		Parent: dir,
	}
	file4 := &analyze.File{
		Name:   "d",
		Usage:  1e3 + 1,
		Parent: dir,
	}
	dir.Files = analyze.Files{file, file2, file3, file4}

	progress.Mutex.Lock()
	progress.Done = true
	progress.Mutex.Unlock()

	return dir
}
