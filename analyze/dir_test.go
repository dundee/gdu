package analyze

import (
	"os"
	"sort"
	"testing"

	"github.com/dundee/gdu/v4/internal/testdir"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir("test_dir", func(_ string) bool { return false })

	assert.True(t, analyzer.GetProgress().Done)
	analyzer.ResetProgress()
	assert.False(t, analyzer.GetProgress().Done)

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7+4096*3), dir.Size)
	assert.Equal(t, 5, dir.ItemCount)
	assert.True(t, dir.IsDir)

	// test dir tree
	assert.Equal(t, "nested", dir.Files[0].Name)
	assert.Equal(t, "subnested", dir.Files[0].Files[1].Name)

	// test file
	assert.Equal(t, "file2", dir.Files[0].Files[0].Name)
	assert.Equal(t, int64(2), dir.Files[0].Files[0].Size)

	assert.Equal(t, "file", dir.Files[0].Files[1].Files[0].Name)
	assert.Equal(t, int64(5), dir.Files[0].Files[1].Files[0].Size)

	// test parent link
	assert.Equal(t, "test_dir", dir.Files[0].Files[1].Files[0].Parent.Parent.Parent.Name)
}

func TestIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dir := CreateAnalyzer().AnalyzeDir("test_dir", func(_ string) bool { return true })

	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, 1, dir.ItemCount)
}

func TestFlags(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	os.Mkdir("test_dir/empty", 0644)

	os.Symlink("test_dir/nested/file2", "test_dir/nested/file3")

	dir := CreateAnalyzer().AnalyzeDir("test_dir", func(_ string) bool { return false })
	sort.Sort(dir.Files)

	assert.Equal(t, int64(28+4096*4), dir.Size)
	assert.Equal(t, 7, dir.ItemCount)

	// test file3
	assert.Equal(t, "nested", dir.Files[0].Name)
	assert.Equal(t, "file3", dir.Files[0].Files[1].Name)
	assert.Equal(t, int64(21), dir.Files[0].Files[1].Size)
	assert.Equal(t, int64(0), dir.Files[0].Files[1].Usage)
	assert.Equal(t, '@', dir.Files[0].Files[1].Flag)

	assert.Equal(t, 'e', dir.Files[1].Flag)
}

func TestHardlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	os.Link("test_dir/nested/file2", "test_dir/nested/file3")

	dir := CreateAnalyzer().AnalyzeDir("test_dir", func(_ string) bool { return false })

	assert.Equal(t, int64(7+4096*3), dir.Size) // file2 and file3 are counted just once for size
	assert.Equal(t, 6, dir.ItemCount)          // but twice for item count

	// test file3
	assert.Equal(t, "file3", dir.Files[0].Files[1].Name)
	assert.Equal(t, int64(2), dir.Files[0].Files[1].Size)
	assert.Equal(t, 'H', dir.Files[0].Files[1].Flag)
}

func TestErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	os.Chmod("test_dir/nested", 0)
	defer os.Chmod("test_dir/nested", 0755)

	dir := CreateAnalyzer().AnalyzeDir("test_dir", func(_ string) bool { return false })

	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, 2, dir.ItemCount)
	assert.Equal(t, '.', dir.Flag)

	assert.Equal(t, "nested", dir.Files[0].Name)
	assert.Equal(t, '!', dir.Files[0].Flag)
}

func BenchmarkAnalyzeDir(b *testing.B) {
	fin := testdir.CreateTestDir()
	defer fin()

	b.ResetTimer()

	CreateAnalyzer().AnalyzeDir("test_dir", func(_ string) bool { return false })
}
