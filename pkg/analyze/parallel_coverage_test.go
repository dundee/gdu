package analyze

import (
	"os"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
)

func TestParallelAnalyzerSetFollowSymlinks(t *testing.T) {
	analyzer := CreateAnalyzer()
	analyzer.SetFollowSymlinks(true)
	assert.True(t, analyzer.followSymlinks)
	analyzer.SetFollowSymlinks(false)
	assert.False(t, analyzer.followSymlinks)
}

func TestParallelAnalyzerSetShowAnnexedSize(t *testing.T) {
	analyzer := CreateAnalyzer()
	analyzer.SetShowAnnexedSize(true)
	assert.True(t, analyzer.gitAnnexedSize)
	analyzer.SetShowAnnexedSize(false)
	assert.False(t, analyzer.gitAnnexedSize)
}

func TestGetDirFlagWithError(t *testing.T) {
	flag := getDirFlag(os.ErrNotExist, 5)
	assert.Equal(t, '!', flag)
}

func TestGetDirFlagWithEmptyDir(t *testing.T) {
	flag := getDirFlag(nil, 0)
	assert.Equal(t, 'e', flag)
}

func TestGetDirFlagWithNormalDir(t *testing.T) {
	flag := getDirFlag(nil, 5)
	assert.Equal(t, ' ', flag)
}

func TestGetFlagWithSymlink(t *testing.T) {
	// Create a temporary symlink
	symlinkPath := "/tmp/test_symlink"
	defer os.Remove(symlinkPath)

	err := os.Symlink("/tmp", symlinkPath)
	assert.NoError(t, err)

	info, err := os.Lstat(symlinkPath)
	assert.NoError(t, err)

	flag := getFlag(info)
	assert.Equal(t, '@', flag)
}

func TestGetFlagWithRegularFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	info, err := os.Stat("test_dir/nested/file2")
	assert.NoError(t, err)

	flag := getFlag(info)
	assert.Equal(t, ' ', flag)
}

func TestParallelAnalyzerUpdateProgress(t *testing.T) {
	analyzer := CreateAnalyzer()

	// Start the progress updater
	go analyzer.updateProgress()

	// Send some progress updates
	analyzer.progressChan <- struct {
		CurrentItemName string
		ItemCount       int
		TotalSize       int64
	}{
		CurrentItemName: "test",
		ItemCount:       5,
		TotalSize:       100,
	}

	// Wait a bit for the progress to be processed
	time.Sleep(10 * time.Millisecond)

	// Send done signal
	analyzer.progressDoneChan <- struct{}{}

	// Wait for the updater to finish
	time.Sleep(10 * time.Millisecond)
}

func TestParallelAnalyzerUpdateProgressWithDefaultCase(t *testing.T) {
	analyzer := CreateAnalyzer()

	// Start the progress updater
	go analyzer.updateProgress()

	// Send some progress updates
	analyzer.progressChan <- struct {
		CurrentItemName string
		ItemCount       int
		TotalSize       int64
	}{
		CurrentItemName: "test",
		ItemCount:       5,
		TotalSize:       100,
	}

	// Wait a bit for the progress to be processed
	time.Sleep(10 * time.Millisecond)

	// Send another progress update to trigger the default case
	analyzer.progressChan <- struct {
		CurrentItemName string
		ItemCount       int
		TotalSize       int64
	}{
		CurrentItemName: "test2",
		ItemCount:       3,
		TotalSize:       50,
	}

	// Wait a bit for the progress to be processed
	time.Sleep(10 * time.Millisecond)

	// Send done signal
	analyzer.progressDoneChan <- struct{}{}

	// Wait for the updater to finish
	time.Sleep(10 * time.Millisecond)
}


func TestParallelAnalyzerAnalyzeDirWithIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(name, _ string) bool { return name == "nested" }, func(_ string) bool { return false },
	).(*Dir)

	analyzer.GetDone().Wait()

	assert.NotNil(t, dir)
	assert.Equal(t, "test_dir", dir.Name)
	// Should have fewer items since nested directory was ignored
	assert.Less(t, dir.ItemCount, 5)
}
