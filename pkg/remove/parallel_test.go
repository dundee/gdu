package remove

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func TestRemoveFileParallel(t *testing.T) {
	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "xxx",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  ".",
	}

	subdir := &analyze.Dir{
		File: &analyze.File{
			Name:   "yyy",
			Size:   4,
			Usage:  8,
			Parent: dir,
		},
		ItemCount: 2,
	}
	file := &analyze.File{
		Name:   "zzz",
		Size:   3,
		Usage:  4,
		Parent: subdir,
	}
	dir.Files = fs.Files{subdir}
	subdir.Files = fs.Files{file}

	err := ItemFromDirParallel(subdir, file)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(subdir.Files))
	assert.Equal(t, 1, subdir.ItemCount)
	assert.Equal(t, int64(1), subdir.Size)
	assert.Equal(t, int64(4), subdir.Usage)
	assert.Equal(t, 1, len(dir.Files))
	assert.Equal(t, 2, dir.ItemCount)
	assert.Equal(t, int64(2), dir.Size)
}

func TestRemoveDirParallel(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := analyze.CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*analyze.Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	subdir := dir.Files[0].(*analyze.Dir)

	err := ItemFromDirParallel(dir, subdir)
	assert.Nil(t, err)
}
