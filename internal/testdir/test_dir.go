package testdir

import (
	"io/fs"
	"os"
)

// CreateTestDir creates test dir structure
func CreateTestDir() func() {
	if err := os.MkdirAll("test_dir/nested/subnested", os.ModePerm); err != nil {
		panic(err)
	}
	if err := os.WriteFile("test_dir/nested/subnested/file", []byte("hello"), 0600); err != nil {
		panic(err)
	}
	if err := os.WriteFile("test_dir/nested/file2", []byte("go"), 0600); err != nil {
		panic(err)
	}
	return func() {
		err := os.RemoveAll("test_dir")
		if err != nil {
			panic(err)
		}
	}
}

// MockedPathChecker is mocked os.Stat, returns (nil, nil)
func MockedPathChecker(path string) (fs.FileInfo, error) {
	return nil, nil
}
