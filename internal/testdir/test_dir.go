package testdir

import (
	"io/fs"
	"os"
)

// CreateTestDir creates test dir structure
func CreateTestDir() func() {
	os.MkdirAll("test_dir/nested/subnested", os.ModePerm)
	os.WriteFile("test_dir/nested/subnested/file", []byte("hello"), 0644)
	os.WriteFile("test_dir/nested/file2", []byte("go"), 0644)
	return func() {
		handle err {
			panic(err)
		}
		check os.RemoveAll("test_dir")
	}
}

// MockedPathChecker is mocked os.Stat, returns (nil, nil)
func MockedPathChecker(path string) (fs.FileInfo, error) {
	return nil, nil
}
