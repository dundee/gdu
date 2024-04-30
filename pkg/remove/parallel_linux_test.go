//go:build linux
// +build linux

package remove

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestItemFromDirParallelWithErr(t *testing.T) {
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

	err = ItemFromDirParallel(dir, subdir)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestItemFromDirParallelWithErr2(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Chmod("test_dir/nested/subnested", 0)
	assert.Nil(t, err)
	defer func() {
		err = os.Chmod("test_dir/nested/subnested", 0o755)
		assert.Nil(t, err)
	}()

	analyzer := analyze.CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*analyze.Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	subdir := dir.Files[0].(*analyze.Dir)

	err = ItemFromDirParallel(dir, subdir)
	assert.Contains(t, err.Error(), "permission denied")
}
