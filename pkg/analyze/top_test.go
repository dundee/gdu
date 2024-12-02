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
		"test_dir", func(_, _ string) bool { return false }, false,
	)

	topFiles := CollectTopFiles(dir, 2)
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
		"test_dir", func(_, _ string) bool { return false }, false,
	)

	topFiles := CollectTopFiles(dir, 1)
	assert.Equal(t, 1, len(topFiles))
	assert.Equal(t, "file", topFiles[0].GetName())
	assert.Equal(t, int64(5), topFiles[0].GetSize())
}

func TestAdd2(t *testing.T) {
	topList := NewTopList(2)
	topList.Add(&File{Size: 1, Name: "file1"})
	topList.Add(&File{Size: 5, Name: "file5"})
	topList.Add(&File{Size: 2, Name: "file2"})

	sort.Sort(sort.Reverse(fs.ByApparentSize(topList.Items)))

	assert.Equal(t, 2, len(topList.Items))
	assert.Equal(t, "file5", topList.Items[0].GetName())
	assert.Equal(t, "file2", topList.Items[1].GetName())
}

func TestAdd3(t *testing.T) {
	topList := NewTopList(3)
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
