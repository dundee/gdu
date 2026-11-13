package analyze

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestStoredAnalyzerGetProgress(t *testing.T) {
	analyzer := createStoredAnalyzer("/tmp/test")
	progress := analyzer.GetProgress()
	assert.Equal(t, int64(0), progress.ItemCount)
}

func TestStoredDirRemoveFile(t *testing.T) {
	// Test RemoveFile functionality
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := createStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{
		IgnoreFile: func(_ string) bool { return false },
	}).(*StoredDir)

	// Remove a file
	files := slices.Collect(dir.GetFiles(fs.SortByName, fs.SortAsc))
	if len(files) > 0 {
		dir.RemoveFile(files[0])
	}
}

func TestStoredDirUpdateStats(t *testing.T) {
	// Test UpdateStats functionality
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := createStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{
		IgnoreFile: func(_ string) bool { return false },
	}).(*StoredDir)

	dir.UpdateStats(false)
}

func TestStoredDirUpdateStatsWithMtimeUpdate(t *testing.T) {
	// Test UpdateStats with mtime updates
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := createStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{
		IgnoreFile: func(_ string) bool { return false },
	}).(*StoredDir)

	// Create a file with newer mtime
	file := &File{
		Name:  "newfile",
		Mtime: time.Now().Add(time.Hour),
	}
	dir.AddFile(file)

	dir.UpdateStats(false)
}

func TestStoredDirUpdateStatsWithFlagUpdate(t *testing.T) {
	// Test UpdateStats with flag updates
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := createStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{
		IgnoreFile: func(_ string) bool { return false },
	}).(*StoredDir)

	// Create a file with error flag
	file := &File{
		Name: "errorfile",
		Flag: '!',
	}
	dir.AddFile(file)

	dir.UpdateStats(false)
	// Just test that UpdateStats runs without error
	// The flag behavior depends on the specific implementation
}

func TestStoredDirUpdateStatsWithDotFlag(t *testing.T) {
	// Test UpdateStats with dot flag
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := createStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{
		IgnoreFile: func(_ string) bool { return false },
	}).(*StoredDir)

	// Create a file with dot flag
	file := &File{
		Name: "dotfile",
		Flag: '.',
	}
	dir.AddFile(file)

	dir.UpdateStats(false)
	assert.Equal(t, '.', dir.Flag)
}

func TestStoredAnalyzerWithZip(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	// Create valid zip
	createTestZipFile(t, "test_dir/valid.zip")

	// Create invalid zip
	f, err := os.Create("test_dir/invalid.zip")
	assert.NoError(t, err)
	_, err = f.WriteString("this is not a zip file")
	assert.NoError(t, err)
	f.Close()

	analyzer := createStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{
		IgnoreFile:      func(_ string) bool { return false },
		ArchiveBrowsing: true,
	}).(*StoredDir)

	// Check valid.zip
	var validZip fs.Item
	var invalidZip fs.Item

	for file := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		if file.GetName() == "valid.zip" {
			validZip = file
		}
		if file.GetName() == "invalid.zip" {
			invalidZip = file
		}
	}

	assert.NotNil(t, validZip)
	assert.True(t, validZip.IsDir())
	assert.Greater(t, validZip.GetSize(), int64(0))

	assert.NotNil(t, invalidZip)
	assert.False(t, invalidZip.IsDir())
	assert.Equal(t, int64(22), invalidZip.GetSize())
}
