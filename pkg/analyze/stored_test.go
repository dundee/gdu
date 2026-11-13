package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoredAnalyzer(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// disk usage depends on the filesystem block size, take it from stat
	info, err := os.Lstat("test_dir/nested/file2")
	assert.NoError(t, err)
	wantUsage, _ := getPlatformSpecificUsageAndMli(info, "test_dir/nested/file2")

	a := CreateStoredAnalyzer("/tmp/badger")
	dir := a.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*StoredDir)

	a.GetDone().Wait()

	dir.UpdateStatsWithFileFiltering(make(fs.HardLinkedItems))

	// test dir info; must match the parallel analyzer on the same fixture
	// (see TestAnalyzeDir in dir_test.go)
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7), dir.Size)
	assert.Equal(t, int64(5), dir.ItemCount)
	assert.True(t, dir.IsDir())

	// test dir tree
	files := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, "nested", files[0].GetName())

	nested := files[0].(*StoredDir)
	nestedFiles := slices.Collect(nested.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, "subnested", nestedFiles[1].GetName())

	// test file
	assert.Equal(t, "file2", nestedFiles[0].GetName())
	assert.Equal(t, int64(2), nestedFiles[0].GetSize())
	assert.Equal(t, wantUsage, nestedFiles[0].GetUsage())

	subnested := nestedFiles[1].(*StoredDir)
	subnestedFiles := slices.Collect(subnested.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Equal(t, "file", subnestedFiles[0].GetName())
	assert.Equal(t, int64(5), subnestedFiles[0].GetSize())
}

func TestStoredAnalyzerLoadAnalysis(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	storagePath := filepath.Join(t.TempDir(), "db.badger")
	scanned := CreateStoredAnalyzer(storagePath).AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)
	// computing the totals is what stores them
	scanned.UpdateStats(make(fs.HardLinkedItems))

	// changes made since the scan are not in the stored analysis
	require.NoError(t, os.WriteFile("test_dir/nested/added", []byte("new"), 0o600))

	loaded, err := CreateStoredAnalyzer(storagePath).LoadAnalysis("test_dir")
	require.NoError(t, err)
	assert.Equal(t, "test_dir", loaded.GetName())
	assert.Equal(t, int64(5), loaded.GetItemCount())

	_, err = CreateStoredAnalyzer(storagePath).LoadAnalysis("other_dir")
	assert.ErrorContains(t, err, "holds no analysis of other_dir")
}

func TestRemoveStoredFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateStoredAnalyzer("/tmp/badger")
	dir := a.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*StoredDir)

	a.GetDone().Wait()
	a.ResetProgress()

	dir.UpdateStats(make(fs.HardLinkedItems))

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7), dir.Size)
	assert.Equal(t, int64(5), dir.ItemCount)
	assert.True(t, dir.IsDir())

	dirFiles := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))
	subdir := dirFiles[0].(*StoredDir)
	subdirFiles := slices.Collect(subdir.GetFiles(fs.SortByName, fs.SortAsc))
	subdir.RemoveFile(subdirFiles[0])

	closeFn := a.storage.Open()
	defer closeFn()
	stored, err := a.storage.GetDirForPath("test_dir")
	assert.NoError(t, err)

	assert.Equal(t, int64(4), stored.GetItemCount())
	assert.Equal(t, int64(5), stored.GetSize())

	storedFiles := slices.Collect(stored.GetFiles(fs.SortByName, fs.SortAsc))
	storedNested := storedFiles[0].(*StoredDir)
	storedNestedFiles := slices.Collect(storedNested.GetFiles(fs.SortByName, fs.SortAsc))
	storedSubnested := storedNestedFiles[0].(*StoredDir)
	storedSubnestedFiles := slices.Collect(storedSubnested.GetFiles(fs.SortByName, fs.SortAsc))
	file := storedSubnestedFiles[0]
	assert.Equal(t, false, file.IsDir())
	assert.Equal(t, "file", file.GetName())
	assert.Equal(t, "test_dir/nested/subnested", file.GetParent().GetPath())
}
