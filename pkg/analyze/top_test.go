package analyze

import (
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
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
