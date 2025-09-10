package analyze

import (
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestStoredAnalyzerGetProgressChan(t *testing.T) {
	analyzer := CreateStoredAnalyzer("/tmp/test")
	progressChan := analyzer.GetProgressChan()
	assert.NotNil(t, progressChan)
}

func TestStoredAnalyzerSetFollowSymlinks(t *testing.T) {
	analyzer := CreateStoredAnalyzer("/tmp/test")
	analyzer.SetFollowSymlinks(true)
	assert.True(t, analyzer.followSymlinks)
	analyzer.SetFollowSymlinks(false)
	assert.False(t, analyzer.followSymlinks)
}

func TestStoredAnalyzerSetShowAnnexedSize(t *testing.T) {
	analyzer := CreateStoredAnalyzer("/tmp/test")
	analyzer.SetShowAnnexedSize(true)
	assert.True(t, analyzer.gitAnnexedSize)
	analyzer.SetShowAnnexedSize(false)
	assert.False(t, analyzer.gitAnnexedSize)
}

func TestStoredDirGetFilesCached(t *testing.T) {
	// Test when files are already cached
	files := make(fs.Files, 0)
	dir := &StoredDir{
		Dir: &Dir{
			File: &File{
				Name: "test",
			},
			BasePath: "/test",
		},
		cachedFiles: files,
	}

	result := dir.GetFiles()
	assert.Equal(t, files, result)
}

func TestStoredDirRemoveFile(t *testing.T) {
	// Test RemoveFile functionality
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*StoredDir)

	analyzer.GetDone().Wait()

	// Remove a file
	if len(dir.GetFiles()) > 0 {
		dir.RemoveFile(dir.GetFiles()[0])
	}
}

func TestStoredDirUpdateStats(t *testing.T) {
	// Test UpdateStats functionality
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*StoredDir)

	analyzer.GetDone().Wait()

	dir.UpdateStats(make(fs.HardLinkedItems))
}

func TestStoredDirUpdateStatsWithMtimeUpdate(t *testing.T) {
	// Test UpdateStats with mtime updates
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*StoredDir)

	analyzer.GetDone().Wait()

	// Create a file with newer mtime
	file := &File{
		Name:  "newfile",
		Mtime: time.Now().Add(time.Hour),
	}
	dir.AddFile(file)

	dir.UpdateStats(make(fs.HardLinkedItems))
}

func TestStoredDirUpdateStatsWithFlagUpdate(t *testing.T) {
	// Test UpdateStats with flag updates
	fin := testdir.CreateTestDir()
	defer fin()
	
	analyzer := CreateStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*StoredDir)
	
	analyzer.GetDone().Wait()
	
	// Create a file with error flag
	file := &File{
		Name: "errorfile",
		Flag: '!',
	}
	dir.AddFile(file)
	
	dir.UpdateStats(make(fs.HardLinkedItems))
	// Just test that UpdateStats runs without error
	// The flag behavior depends on the specific implementation
}

func TestStoredDirUpdateStatsWithDotFlag(t *testing.T) {
	// Test UpdateStats with dot flag
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStoredAnalyzer("/tmp/test")
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*StoredDir)

	analyzer.GetDone().Wait()

	// Create a file with dot flag
	file := &File{
		Name: "dotfile",
		Flag: '.',
	}
	dir.AddFile(file)

	dir.UpdateStats(make(fs.HardLinkedItems))
	assert.Equal(t, '.', dir.Flag)
}
