package analyze

import (
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
)

func TestSequentialAnalyzerSetFollowSymlinks(t *testing.T) {
	analyzer := CreateSeqAnalyzer()
	analyzer.SetFollowSymlinks(true)
	assert.True(t, analyzer.followSymlinks)
	analyzer.SetFollowSymlinks(false)
	assert.False(t, analyzer.followSymlinks)
}

func TestSequentialAnalyzerSetShowAnnexedSize(t *testing.T) {
	analyzer := CreateSeqAnalyzer()
	analyzer.SetShowAnnexedSize(true)
	assert.True(t, analyzer.gitAnnexedSize)
	analyzer.SetShowAnnexedSize(false)
	assert.False(t, analyzer.gitAnnexedSize)
}

func TestSequentialAnalyzerUpdateProgress(t *testing.T) {
	analyzer := CreateSeqAnalyzer()

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

func TestSequentialAnalyzerUpdateProgressWithDefaultCase(t *testing.T) {
	analyzer := CreateSeqAnalyzer()

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

func TestSequentialAnalyzerAnalyzeDirWithIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateSeqAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(name, _ string) bool { return name == "nested" }, func(_ string) bool { return false },
	).(*Dir)

	analyzer.GetDone().Wait()

	assert.NotNil(t, dir)
	assert.Equal(t, "test_dir", dir.Name)
	// Should have fewer items since nested directory was ignored
	assert.Less(t, dir.ItemCount, 5)
}
