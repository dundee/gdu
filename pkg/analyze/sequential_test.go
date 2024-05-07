package analyze

import (
	"os"
	"sort"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestAnalyzeDirSeq(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateSeqAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*Dir)

	progress := <-analyzer.GetProgressChan()
	assert.GreaterOrEqual(t, progress.TotalSize, int64(0))

	analyzer.GetDone().Wait()
	analyzer.ResetProgress()
	dir.UpdateStats(make(fs.HardLinkedItems))

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7+4096*3), dir.Size)
	assert.Equal(t, 5, dir.ItemCount)
	assert.True(t, dir.IsDir())

	// test dir tree
	assert.Equal(t, "nested", dir.Files[0].GetName())
	assert.Equal(t, "subnested", dir.Files[0].(*Dir).Files[1].GetName())

	// test file
	assert.Equal(t, "file2", dir.Files[0].(*Dir).Files[0].GetName())
	assert.Equal(t, int64(2), dir.Files[0].(*Dir).Files[0].GetSize())

	assert.Equal(
		t, "file", dir.Files[0].(*Dir).Files[1].(*Dir).Files[0].GetName(),
	)
	assert.Equal(
		t, int64(5), dir.Files[0].(*Dir).Files[1].(*Dir).Files[0].GetSize(),
	)

	// test parent link
	assert.Equal(
		t,
		"test_dir",
		dir.Files[0].(*Dir).
			Files[1].(*Dir).
			Files[0].
			GetParent().
			GetParent().
			GetParent().
			GetName(),
	)
}

func TestIgnoreDirSeq(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dir := CreateSeqAnalyzer().AnalyzeDir(
		"test_dir", func(_, _ string) bool { return true }, false,
	).(*Dir)

	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, 1, dir.ItemCount)
}

func TestFlagsSeq(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink("test_dir/nested/file2", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateSeqAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(28+4096*4), dir.Size)
	assert.Equal(t, 7, dir.ItemCount)

	// test file3
	assert.Equal(t, "nested", dir.Files[0].GetName())
	assert.Equal(t, "file3", dir.Files[0].(*Dir).Files[1].GetName())
	assert.Equal(t, int64(21), dir.Files[0].(*Dir).Files[1].GetSize())
	assert.Equal(t, '@', dir.Files[0].(*Dir).Files[1].GetFlag())

	assert.Equal(t, 'e', dir.Files[1].GetFlag())
}

func TestHardlinkSeq(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Link("test_dir/nested/file2", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateSeqAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, int64(7+4096*3), dir.Size) // file2 and file3 are counted just once for size
	assert.Equal(t, 6, dir.ItemCount)          // but twice for item count

	// test file3
	assert.Equal(t, "file3", dir.Files[0].(*Dir).Files[1].GetName())
	assert.Equal(t, int64(2), dir.Files[0].(*Dir).Files[1].GetSize())
	assert.Equal(t, 'H', dir.Files[0].(*Dir).Files[1].GetFlag())
}

func TestFollowSymlinkSeq(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink("./file2", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateSeqAnalyzer()
	analyzer.SetFollowSymlinks(true)
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(9+4096*4), dir.Size)
	assert.Equal(t, 7, dir.ItemCount)

	// test file3
	assert.Equal(t, "nested", dir.Files[0].GetName())
	assert.Equal(t, "file3", dir.Files[0].(*Dir).Files[1].GetName())
	assert.Equal(t, int64(2), dir.Files[0].(*Dir).Files[1].GetSize())
	assert.Equal(t, ' ', dir.Files[0].(*Dir).Files[1].GetFlag())

	assert.Equal(t, 'e', dir.Files[1].GetFlag())
}

func TestBrokenSymlinkSkippedSeq(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink("xxx", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateSeqAnalyzer()
	analyzer.SetFollowSymlinks(true)
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(7+4096*4), dir.Size)
	assert.Equal(t, 6, dir.ItemCount)

	assert.Equal(t, '!', dir.Files[0].GetFlag())
}

func BenchmarkAnalyzeDirSeq(b *testing.B) {
	fin := testdir.CreateTestDir()
	defer fin()

	b.ResetTimer()

	analyzer := CreateSeqAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))
}
