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

func TestAnalyzeDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*Dir)

	progress := analyzer.GetProgress()
	assert.GreaterOrEqual(t, progress.TotalUsage, int64(0))

	analyzer.GetDone().Wait()
	analyzer.ResetProgress()
	dir.UpdateStats(make(fs.HardLinkedItems))

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7), dir.Size)
	assert.Equal(t, int64(5), dir.ItemCount)
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

func TestIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dir := CreateAnalyzer().AnalyzeDir(
		"test_dir", func(_, _ string) bool { return true }, func(_ string) bool { return false },
	).(*Dir)

	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(1), dir.ItemCount)
}

func TestFlags(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink("test_dir/nested/file2", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(28+512), dir.Size)
	assert.Equal(t, int64(7), dir.ItemCount)

	// test file3
	assert.Equal(t, "nested", dir.Files[0].GetName())
	assert.Equal(t, "file3", dir.Files[0].(*Dir).Files[1].GetName())
	assert.Equal(t, int64(21), dir.Files[0].(*Dir).Files[1].GetSize())
	assert.Equal(t, '@', dir.Files[0].(*Dir).Files[1].GetFlag())

	assert.Equal(t, 'e', dir.Files[1].GetFlag())
}

func TestHardlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Link("test_dir/nested/file2", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, int64(7), dir.Size)      // file2 and file3 are counted just once for size
	assert.Equal(t, int64(6), dir.ItemCount) // but twice for item count

	// test file3
	assert.Equal(t, "file3", dir.Files[0].(*Dir).Files[1].GetName())
	assert.Equal(t, int64(2), dir.Files[0].(*Dir).Files[1].GetSize())
	assert.Equal(t, 'H', dir.Files[0].(*Dir).Files[1].GetFlag())
}

func TestFollowSymlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink("./file2", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateAnalyzer()
	analyzer.SetFollowSymlinks(true)
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(9+512), dir.Size)
	assert.Equal(t, int64(7), dir.ItemCount)

	// test file3
	assert.Equal(t, "nested", dir.Files[0].GetName())
	assert.Equal(t, "file3", dir.Files[0].(*Dir).Files[1].GetName())
	assert.Equal(t, int64(2), dir.Files[0].(*Dir).Files[1].GetSize())
	assert.Equal(t, ' ', dir.Files[0].(*Dir).Files[1].GetFlag())

	assert.Equal(t, 'e', dir.Files[1].GetFlag())
}

func TestGitAnnexSymlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink(
		".git/annex/objects/qx/qX/SHA256E-s967858083--"+
			"3e54803fded8dc3a9ea68b106f7b51e04e33c79b4a7b32a860f0b22d89af5c65.mp4/SHA256E-s967858083--"+
			"3e54803fded8dc3a9ea68b106f7b51e04e33c79b4a7b32a860f0b22d89af5c65.mp4",
		"test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateAnalyzer()
	analyzer.SetFollowSymlinks(true)
	analyzer.SetShowAnnexedSize(true)
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(967858083+7+512), dir.Size)
	assert.Equal(t, int64(7), dir.ItemCount)

	// test file3
	assert.Equal(t, "nested", dir.Files[0].GetName())
	assert.Equal(t, "file3", dir.Files[0].(*Dir).Files[1].GetName())
	assert.Equal(t, int64(967858083), dir.Files[0].(*Dir).Files[1].GetSize())
	assert.Equal(t, '@', dir.Files[0].(*Dir).Files[1].GetFlag())

	assert.Equal(t, 'e', dir.Files[1].GetFlag())
}

func TestBrokenSymlinkSkipped(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink("xxx", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateAnalyzer()
	analyzer.SetFollowSymlinks(true)
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(7+512), dir.Size)
	assert.Equal(t, int64(6), dir.ItemCount)

	assert.Equal(t, '!', dir.Files[0].GetFlag())
}

func BenchmarkAnalyzeDir(b *testing.B) {
	fin := testdir.CreateTestDir()
	defer fin()

	b.ResetTimer()

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))
}

func TestParallelVsSequentialConsistency(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// Run sequential analyzer
	seqAnalyzer := CreateSeqAnalyzer()
	seqDir := seqAnalyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)
	seqAnalyzer.GetDone().Wait()
	seqDir.UpdateStats(make(fs.HardLinkedItems))
	seqNames := getFileNames(seqDir)

	// Run parallel analyzer
	parAnalyzer := CreateAnalyzer()
	parDir := parAnalyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)
	parAnalyzer.GetDone().Wait()
	parDir.UpdateStats(make(fs.HardLinkedItems))
	parNames := getFileNames(parDir)

	// Results should match
	assert.Equal(t, seqNames, parNames,
		"Parallel and sequential analyzers produced different results")
}

// getFileNames recursively collects file names from a directory tree
func getFileNames(item fs.Item) []string {
	names := []string{item.GetName()}
	if item.IsDir() {
		for child := range item.GetFiles(fs.SortByName, fs.SortAsc) {
			names = append(names, getFileNames(child)...)
		}
	}
	return names
}
