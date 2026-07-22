package analyze

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	go analyzer.UpdateProgress()

	// Set some progress via atomics
	analyzer.progressCurrentItemName.Store("test")
	analyzer.progressItemCount.Add(5)
	analyzer.progressTotalUsage.Add(100)

	// Wait a bit for the progress to be processed
	time.Sleep(100 * time.Millisecond)

	// Send done signal
	analyzer.progressDoneChan <- struct{}{}

	// Wait for the updater to finish
	time.Sleep(10 * time.Millisecond)
}

func TestParallelAnalyzerUpdateProgressWithDefaultCase(t *testing.T) {
	analyzer := CreateAnalyzer()

	// Start the progress updater
	go analyzer.UpdateProgress()

	// Set some progress via atomics
	analyzer.progressCurrentItemName.Store("test")
	analyzer.progressItemCount.Add(5)
	analyzer.progressTotalUsage.Add(100)

	// Wait a bit for the progress to be processed
	time.Sleep(100 * time.Millisecond)

	// Update progress again
	analyzer.progressCurrentItemName.Store("test2")
	analyzer.progressItemCount.Add(3)
	analyzer.progressTotalUsage.Store(50)

	// Wait a bit for the progress to be processed
	time.Sleep(100 * time.Millisecond)

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
	assert.Less(t, dir.ItemCount, int64(5))
}

func TestAnalyzersMarkFilesystemErrors(t *testing.T) {
	factories := []struct {
		name string
		new  func() common.Analyzer
	}{
		{name: "parallel", new: func() common.Analyzer { return CreateAnalyzer() }},
		{name: "stable parallel", new: func() common.Analyzer { return CreateStableOrderAnalyzer() }},
		{name: "sequential", new: func() common.Analyzer { return CreateSeqAnalyzer() }},
	}

	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			t.Run("disappeared file", func(t *testing.T) {
				root := t.TempDir()
				filePath := filepath.Join(root, "removed-during-scan")
				require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o600))

				var removeErr error
				dir := factory.new().AnalyzeDir(
					root,
					func(_, _ string) bool { return false },
					func(string) bool {
						removeErr = os.Remove(filePath)
						return false
					},
				)

				require.NoError(t, removeErr)
				assert.Equal(t, '!', dir.GetFlag())
			})

			t.Run("broken symlink", func(t *testing.T) {
				root := t.TempDir()
				err := os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(root, "broken-link"))
				if err != nil {
					t.Skipf("creating symlink: %v", err)
				}

				analyzer := factory.new()
				analyzer.SetFollowSymlinks(true)
				dir := analyzer.AnalyzeDir(
					root,
					func(_, _ string) bool { return false },
					func(string) bool { return false },
				)

				assert.Equal(t, '!', dir.GetFlag())
			})
		})
	}
}
