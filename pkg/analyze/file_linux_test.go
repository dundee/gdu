//go:build linux
// +build linux

package analyze

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
)

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
