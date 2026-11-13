package analyze

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestTopDirAnalyzeDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{IgnoreFile: func(_ string) bool { return false }})

	assert.Equal(t, "test_dir", dir.GetName())

	simpleDir := dir.(*SimpleDir)
	simpleDir.UpdateStats(false)

	// Should have one top-level entry: "nested" directory
	assert.Equal(t, 1, len(simpleDir.Files))
	assert.True(t, simpleDir.Files[0].IsDir)
	assert.Equal(t, "nested", simpleDir.Files[0].Name)
	// nested dir contains: file2 (2 bytes) + subnested/file (5 bytes) + dir overhead
	assert.Greater(t, simpleDir.Files[0].Size, int64(0))
	assert.Greater(t, simpleDir.Files[0].ItemCount, int64(0))
}

func TestTopDirAnalyzeDirWithFiles(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(tmpDir+"/file_a", []byte("hello"), 0o644)
	assert.Nil(t, err)
	err = os.Mkdir(tmpDir+"/subdir", 0o755)
	assert.Nil(t, err)
	err = os.WriteFile(tmpDir+"/subdir/file_b", []byte("world!"), 0o644)
	assert.Nil(t, err)

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), tmpDir, common.ScanOptions{IgnoreFile: func(_ string) bool { return false }})

	simpleDir := dir.(*SimpleDir)
	simpleDir.UpdateStats(false)

	// Should have 2 entries: file_a and subdir
	assert.Equal(t, 2, len(simpleDir.Files))

	// Verify total size includes both
	assert.Greater(t, simpleDir.GetSize(), int64(0))
}

func TestTopDirAnalyzeDirIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{
		IgnoreDir:  func(name, _ string) bool { return name == "nested" },
		IgnoreFile: func(_ string) bool { return false },
	})

	simpleDir := dir.(*SimpleDir)
	// "nested" directory should be ignored, no entries
	assert.Equal(t, 0, len(simpleDir.Files))
}

func TestTopDirAnalyzeDirIgnoreFileType(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(tmpDir+"/keep.txt", []byte("keep"), 0o644)
	assert.Nil(t, err)
	err = os.WriteFile(tmpDir+"/skip.log", []byte("skip"), 0o644)
	assert.Nil(t, err)

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), tmpDir, common.ScanOptions{IgnoreFile: func(name string) bool {
		return len(name) > 4 && name[len(name)-4:] == ".log"
	}})

	simpleDir := dir.(*SimpleDir)
	assert.Equal(t, 1, len(simpleDir.Files))
	assert.Equal(t, "keep.txt", simpleDir.Files[0].Name)
}

func TestTopDirAnalyzeDirIgnoreFileTypeInSubDir(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.Mkdir(tmpDir+"/sub", 0o755)
	assert.Nil(t, err)
	err = os.WriteFile(tmpDir+"/sub/keep.txt", []byte("keep"), 0o644)
	assert.Nil(t, err)
	err = os.WriteFile(tmpDir+"/sub/skip.log", []byte("skip"), 0o644)
	assert.Nil(t, err)

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), tmpDir, common.ScanOptions{IgnoreFile: func(name string) bool {
		return len(name) > 4 && name[len(name)-4:] == ".log"
	}})

	simpleDir := dir.(*SimpleDir)
	// sub directory should exist but only count keep.txt
	assert.Equal(t, 1, len(simpleDir.Files))
	assert.Equal(t, "sub", simpleDir.Files[0].Name)
	// ItemCount should reflect only the kept file (plus the dir itself)
	assert.Equal(t, int64(2), simpleDir.Files[0].ItemCount)
}

func TestTopDirAnalyzeDirProgress(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	_ = analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{IgnoreFile: func(_ string) bool { return false }})

	// Just verify the progress channel is accessible and was used
	assert.NotNil(t, analyzer.GetProgress())
}

// An analyzer can scan again, each scan reporting its own progress.
func TestTopDirAnalyzeDirTwice(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	_ = analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{IgnoreFile: func(_ string) bool { return false }})

	progress := analyzer.GetProgress()
	assert.GreaterOrEqual(t, progress.TotalUsage, int64(0))

	// Analyze again
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{IgnoreFile: func(_ string) bool { return false }})

	assert.Equal(t, "test_dir", dir.GetName())
}

func TestTopDirAnalyzeDirNonExistent(t *testing.T) {
	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), "/nonexistent_path_xyz", common.ScanOptions{
		IgnoreFile: func(_ string) bool { return false },
	})

	simpleDir := dir.(*SimpleDir)
	assert.Equal(t, "nonexistent_path_xyz", simpleDir.Name)
	assert.Equal(t, 0, len(simpleDir.Files))
}

func TestTopDirAnalyzeDirIgnoreSubDir(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.MkdirAll(tmpDir+"/top/ignored", 0o755)
	assert.Nil(t, err)
	err = os.MkdirAll(tmpDir+"/top/kept", 0o755)
	assert.Nil(t, err)
	err = os.WriteFile(tmpDir+"/top/ignored/file", []byte("data"), 0o644)
	assert.Nil(t, err)
	err = os.WriteFile(tmpDir+"/top/kept/file", []byte("data"), 0o644)
	assert.Nil(t, err)

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), tmpDir, common.ScanOptions{
		IgnoreDir:  func(name, _ string) bool { return name == "ignored" },
		IgnoreFile: func(_ string) bool { return false },
	})

	simpleDir := dir.(*SimpleDir)
	assert.Equal(t, 1, len(simpleDir.Files))
	assert.Equal(t, "top", simpleDir.Files[0].Name)
}

func topDirFlags(t *testing.T, path string) map[string]rune {
	t.Helper()

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), path, common.ScanOptions{IgnoreFile: func(_ string) bool { return false }})

	flags := make(map[string]rune)
	for file := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		flags[file.GetName()] = file.GetFlag()
	}
	return flags
}

func TestTopDirAnalyzeDirEmptyDirFlag(t *testing.T) {
	tmpDir := t.TempDir()
	assert.Nil(t, os.Mkdir(filepath.Join(tmpDir, "empty"), 0o755))
	assert.Nil(t, os.MkdirAll(filepath.Join(tmpDir, "withempty", "sub"), 0o755))

	flags := topDirFlags(t, tmpDir)

	assert.Equal(t, 'e', flags["empty"])
	assert.Equal(t, ' ', flags["withempty"])
}

func TestTopDirAnalyzeDirErrorFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not make directories unreadable on Windows")
	}

	tmpDir := t.TempDir()
	locked := filepath.Join(tmpDir, "locked")
	nestedLocked := filepath.Join(tmpDir, "outer", "locked")
	for _, path := range []string{locked, nestedLocked} {
		assert.Nil(t, os.MkdirAll(path, 0o755))
		assert.Nil(t, os.WriteFile(filepath.Join(path, "file"), []byte("x"), 0o600))
		assert.Nil(t, os.Chmod(path, 0))
		defer os.Chmod(path, 0o755)
	}

	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("test user can still read chmod 000 directory")
	}

	flags := topDirFlags(t, tmpDir)

	assert.Equal(t, '!', flags["locked"])
	assert.Equal(t, '.', flags["outer"])
}

func TestTopDirGetFiles(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{IgnoreFile: func(_ string) bool { return false }})

	simpleDir := dir.(*SimpleDir)
	simpleDir.UpdateStats(false)

	count := 0
	for range simpleDir.GetFiles(fs.SortBySize, fs.SortAsc) {
		count++
	}
	assert.Equal(t, len(simpleDir.Files), count)
}

func TestTopDirFollowSymlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink("nested/subnested/file", "./test_dir/file2")
	assert.Nil(t, err)

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{
		IgnoreFile:     func(_ string) bool { return false },
		FollowSymlinks: true,
	}).(*SimpleDir)
	dir.UpdateStats(false)

	var files []fs.Item
	for file := range dir.GetFiles(fs.SortBySize, fs.SortDesc) {
		files = append(files, file)
	}

	assert.Equal(t, int64(12+512), dir.Size)
	assert.Equal(t, int64(7), dir.ItemCount)

	// test file3
	assert.Equal(t, "file2", files[1].GetName())
	assert.Equal(t, int64(5), files[1].GetSize())
	assert.Equal(t, ' ', files[1].GetFlag())
}

func TestTopDirFollowNestedSymlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink("subnested/file", "./test_dir/nested/file3")
	assert.Nil(t, err)

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{
		IgnoreFile:     func(_ string) bool { return false },
		FollowSymlinks: true,
	}).(*SimpleDir)
	dir.UpdateStats(false)

	var files []fs.Item
	for file := range dir.GetFiles(fs.SortBySize, fs.SortDesc) {
		files = append(files, file)
	}

	assert.Equal(t, int64(12+512), dir.Size)
	assert.Equal(t, int64(7), dir.ItemCount)
}
