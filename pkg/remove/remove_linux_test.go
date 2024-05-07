//go:build linux
// +build linux

package remove

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/stretchr/testify/assert"
)

func TestRemoveFileWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Chmod("test_dir/nested", 0)
	assert.Nil(t, err)
	defer func() {
		err = os.Chmod("test_dir/nested", 0o755)
		assert.Nil(t, err)
	}()

	dir := &analyze.Dir{
		File: &analyze.File{
			Name: "test_dir",
		},
		BasePath: ".",
	}

	subdir := &analyze.Dir{
		File: &analyze.File{
			Name:   "nested",
			Parent: dir,
		},
	}

	err = ItemFromDir(dir, subdir)
	assert.Contains(t, err.Error(), "permission denied")
}
