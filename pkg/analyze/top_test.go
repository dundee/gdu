package analyze

import (
	"sort"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestCollectTopFiles2(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dir := CreateAnalyzer().AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	topFiles := CollectTopFiles(dir, 2, fs.SortByApparentSize)
	assert.Equal(t, 2, len(topFiles))
	assert.Equal(t, "file", topFiles[0].GetName())
	assert.Equal(t, int64(5), topFiles[0].GetSize())
	assert.Equal(t, "file2", topFiles[1].GetName())
	assert.Equal(t, int64(2), topFiles[1].GetSize())
}

func TestCollectTopFiles1(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dir := CreateAnalyzer().AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	topFiles := CollectTopFiles(dir, 1, fs.SortByApparentSize)
	assert.Equal(t, 1, len(topFiles))
	assert.Equal(t, "file", topFiles[0].GetName())
	assert.Equal(t, int64(5), topFiles[0].GetSize())
}

// createDivergingDir creates a dir where the two size metrics disagree:
// "sparse" has the bigger apparent size, "dense" the bigger disk usage.
func createDivergingDir() *Dir {
	dir := &Dir{
		File:      &File{Name: "root"},
		BasePath:  "/",
		ItemCount: 3,
	}
	dir.AddFile(&File{Name: "sparse", Parent: dir, Size: 10 << 20, Usage: 0})
	dir.AddFile(&File{Name: "dense", Parent: dir, Size: 5 << 20, Usage: 8 << 20})
	return dir
}

func TestCollectTopFilesBySize(t *testing.T) {
	topFiles := CollectTopFiles(createDivergingDir(), 2, fs.SortBySize)

	assert.Equal(t, 2, len(topFiles))
	assert.Equal(t, "dense", topFiles[0].GetName())
	assert.Equal(t, "sparse", topFiles[1].GetName())
}

func TestCollectTopFilesByApparentSize(t *testing.T) {
	topFiles := CollectTopFiles(createDivergingDir(), 2, fs.SortByApparentSize)

	assert.Equal(t, 2, len(topFiles))
	assert.Equal(t, "sparse", topFiles[0].GetName())
	assert.Equal(t, "dense", topFiles[1].GetName())
}

func TestAddBySize(t *testing.T) {
	topList := NewTopList(2, fs.SortBySize)
	topList.Add(&File{Size: 5, Usage: 1, Name: "file1"})
	topList.Add(&File{Size: 1, Usage: 5, Name: "file5"})
	topList.Add(&File{Size: 2, Usage: 2, Name: "file2"})

	sort.Sort(sort.Reverse(topList.Items))

	assert.Equal(t, 2, len(topList.Items))
	assert.Equal(t, "file5", topList.Items[0].GetName())
	assert.Equal(t, "file2", topList.Items[1].GetName())
}

func TestAdd2(t *testing.T) {
	topList := NewTopList(2, fs.SortByApparentSize)
	topList.Add(&File{Size: 1, Name: "file1"})
	topList.Add(&File{Size: 5, Name: "file5"})
	topList.Add(&File{Size: 2, Name: "file2"})

	sort.Sort(sort.Reverse(fs.ByApparentSize(topList.Items)))

	assert.Equal(t, 2, len(topList.Items))
	assert.Equal(t, "file5", topList.Items[0].GetName())
	assert.Equal(t, "file2", topList.Items[1].GetName())
}

func TestAdd3(t *testing.T) {
	topList := NewTopList(3, fs.SortByApparentSize)
	topList.Add(&File{Size: 5, Name: "file5"})
	topList.Add(&File{Size: 1, Name: "file1"})
	topList.Add(&File{Size: 2, Name: "file2"})
	topList.Add(&File{Size: 4, Name: "file4"})
	topList.Add(&File{Size: 3, Name: "file3"})

	sort.Sort(sort.Reverse(fs.ByApparentSize(topList.Items)))

	assert.Equal(t, 3, len(topList.Items))
	assert.Equal(t, "file5", topList.Items[0].GetName())
	assert.Equal(t, "file4", topList.Items[1].GetName())
	assert.Equal(t, "file3", topList.Items[2].GetName())
}
