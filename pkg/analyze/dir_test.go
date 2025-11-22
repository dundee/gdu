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

func TestIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dir := CreateAnalyzer().AnalyzeDir(
		"test_dir", func(_, _ string) bool { return true }, false,
	).(*Dir)

	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, 1, dir.ItemCount)
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

func TestHardlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Link("test_dir/nested/file2", "test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateAnalyzer()
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
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(967858083+7+4096*4), dir.Size)
	assert.Equal(t, 7, dir.ItemCount)

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
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	sort.Sort(sort.Reverse(dir.Files))

	assert.Equal(t, int64(7+4096*4), dir.Size)
	assert.Equal(t, 6, dir.ItemCount)

	assert.Equal(t, '!', dir.Files[0].GetFlag())
}

func BenchmarkAnalyzeDir(b *testing.B) {
	fin := testdir.CreateTestDir()
	defer fin()

	b.ResetTimer()

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))
}

func TestParallelStableOrderAnalyzerDeterminism(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// Run parallel analyzer multiple times and verify results are identical
	var results [][]string
	for i := 0; i < 5; i++ {
		analyzer := CreateStableOrderAnalyzer()
		dir := analyzer.AnalyzeDir(
			"test_dir", func(_, _ string) bool { return false }, false,
		)
		analyzer.GetDone().Wait()
		dir.UpdateStats(make(fs.HardLinkedItems))

		names := getFileNames(dir)
		results = append(results, names)
	}

	// All runs should produce identical results
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i],
			"Parallel analyzer run %d produced different results than run 0", i)
	}
}

func TestParallelVsSequentialConsistency(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// Run sequential analyzer
	seqAnalyzer := CreateSeqAnalyzer()
	seqDir := seqAnalyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	)
	seqAnalyzer.GetDone().Wait()
	seqDir.UpdateStats(make(fs.HardLinkedItems))
	seqNames := getFileNames(seqDir)

	// Run parallel analyzer
	parAnalyzer := CreateStableOrderAnalyzer()
	parDir := parAnalyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	)
	parAnalyzer.GetDone().Wait()
	parDir.UpdateStats(make(fs.HardLinkedItems))
	parNames := getFileNames(parDir)

	// Results should match
	assert.Equal(t, seqNames, parNames,
		"Parallel and sequential analyzers produced different results")
}

func TestFileDirectoryInterleaving(t *testing.T) {
	// Create test directory with interleaved files and directories
	err := os.MkdirAll("test_interleave/aaa_dir", 0755)
	assert.NoError(t, err)
	err = os.WriteFile("test_interleave/bbb_file", []byte("content"), 0644)
	assert.NoError(t, err)
	err = os.MkdirAll("test_interleave/ccc_dir", 0755)
	assert.NoError(t, err)
	err = os.WriteFile("test_interleave/ddd_file", []byte("content"), 0644)
	assert.NoError(t, err)
	defer os.RemoveAll("test_interleave")

	// Run sequential analyzer
	seqAnalyzer := CreateSeqAnalyzer()
	seqDir := seqAnalyzer.AnalyzeDir(
		"test_interleave", func(_, _ string) bool { return false }, false,
	).(*Dir)
	seqAnalyzer.GetDone().Wait()

	// Run parallel analyzer
	parAnalyzer := CreateStableOrderAnalyzer()
	parDir := parAnalyzer.AnalyzeDir(
		"test_interleave", func(_, _ string) bool { return false }, false,
	).(*Dir)
	parAnalyzer.GetDone().Wait()

	// Extract file/dir names in order
	seqOrder := make([]string, len(seqDir.Files))
	for i, item := range seqDir.Files {
		seqOrder[i] = item.GetName()
	}

	parOrder := make([]string, len(parDir.Files))
	for i, item := range parDir.Files {
		parOrder[i] = item.GetName()
	}

	// The order must be identical: [aaa_dir, bbb_file, ccc_dir, ddd_file]
	assert.Equal(t, seqOrder, parOrder,
		"Parallel analyzer did not preserve file/directory interleaving")

	// Verify the expected order (alphabetical from os.ReadDir)
	assert.Equal(t, "aaa_dir", seqOrder[0])
	assert.Equal(t, "bbb_file", seqOrder[1])
	assert.Equal(t, "ccc_dir", seqOrder[2])
	assert.Equal(t, "ddd_file", seqOrder[3])
}

// getFileNames recursively collects file names from a directory tree
func getFileNames(item fs.Item) []string {
	names := []string{item.GetName()}
	if item.IsDir() {
		for _, child := range item.GetFiles() {
			names = append(names, getFileNames(child)...)
		}
	}
	return names
}
