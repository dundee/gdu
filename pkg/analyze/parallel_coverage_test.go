package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestParallelAnalyzerAnalyzeDirWithIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", Options{
		IgnoreDir:  func(name, _ string) bool { return name == "nested" },
		IgnoreFile: func(_ string) bool { return false },
	}).(*Dir)

	assert.NotNil(t, dir)
	assert.Equal(t, "test_dir", dir.Name)
	// Should have fewer items since nested directory was ignored
	assert.Less(t, dir.ItemCount, int64(5))
}

func TestAnalyzersMarkFilesystemErrors(t *testing.T) {
	factories := []struct {
		name string
		new  func() Analyzer
	}{
		{name: "parallel", new: func() Analyzer { return CreateAnalyzer() }},
		{name: "sequential", new: func() Analyzer { return CreateSeqAnalyzer() }},
	}

	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			t.Run("disappeared file", func(t *testing.T) {
				root := t.TempDir()
				filePath := filepath.Join(root, "removed-during-scan")
				require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o600))

				var removeErr error
				dir := factory.new().AnalyzeDir(context.Background(), root, Options{IgnoreFile: func(string) bool {
					removeErr = os.Remove(filePath)
					return false
				}})

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
				dir := analyzer.AnalyzeDir(context.Background(), root, Options{
					IgnoreFile:     func(string) bool { return false },
					FollowSymlinks: true,
				})

				assert.Equal(t, '!', dir.GetFlag())
			})
		})
	}
}
