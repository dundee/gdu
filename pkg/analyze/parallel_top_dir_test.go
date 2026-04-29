package analyze

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestTopDirAnalyzeDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	analyzer.GetDone().Wait()

	assert.Equal(t, "test_dir", dir.GetName())

	simpleDir := dir.(*SimpleDir)
	simpleDir.UpdateStats(make(fs.HardLinkedItems))

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
	dir := analyzer.AnalyzeDir(
		tmpDir, func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	analyzer.GetDone().Wait()

	simpleDir := dir.(*SimpleDir)
	simpleDir.UpdateStats(make(fs.HardLinkedItems))

	// Should have 2 entries: file_a and subdir
	assert.Equal(t, 2, len(simpleDir.Files))

	// Verify total size includes both
	assert.Greater(t, simpleDir.GetSize(), int64(0))
}

func TestTopDirAnalyzeDirIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir",
		func(name, _ string) bool { return name == "nested" },
		func(_ string) bool { return false },
	)

	analyzer.GetDone().Wait()

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
	dir := analyzer.AnalyzeDir(
		tmpDir,
		func(_, _ string) bool { return false },
		func(name string) bool {
			return len(name) > 4 && name[len(name)-4:] == ".log"
		},
	)

	analyzer.GetDone().Wait()

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
	dir := analyzer.AnalyzeDir(
		tmpDir,
		func(_, _ string) bool { return false },
		func(name string) bool {
			return len(name) > 4 && name[len(name)-4:] == ".log"
		},
	)

	analyzer.GetDone().Wait()

	simpleDir := dir.(*SimpleDir)
	// sub directory should exist but only count keep.txt
	assert.Equal(t, 1, len(simpleDir.Files))
	assert.Equal(t, "sub", simpleDir.Files[0].Name)
	// ItemCount should reflect only the kept file + the dir itself
	assert.Equal(t, int64(2), simpleDir.Files[0].ItemCount)
}

func TestTopDirAnalyzeDirProgress(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	_ = analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	analyzer.GetDone().Wait()

	// Just verify the progress channel is accessible and was used
	assert.NotNil(t, analyzer.GetProgress())
}

func TestTopDirAnalyzeDirResetProgress(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	_ = analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	progress := analyzer.GetProgress()
	assert.GreaterOrEqual(t, progress.TotalUsage, int64(0))

	analyzer.GetDone().Wait()
	analyzer.ResetProgress()

	// Analyze again after reset
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	analyzer.GetDone().Wait()

	assert.Equal(t, "test_dir", dir.GetName())
}

func TestTopDirAnalyzeDirNonExistent(t *testing.T) {
	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(
		"/nonexistent_path_xyz", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	analyzer.GetDone().Wait()

	simpleDir := dir.(*SimpleDir)
	assert.Equal(t, "nonexistent_path_xyz", simpleDir.Name)
	assert.Equal(t, 0, len(simpleDir.Files))
}

func TestTopDirAnalyzerSetters(t *testing.T) {
	analyzer := CreateTopDirAnalyzer()

	analyzer.SetFollowSymlinks(true)
	assert.True(t, analyzer.followSymlinks)

	analyzer.SetShowAnnexedSize(true)
	assert.True(t, analyzer.gitAnnexedSize)

	analyzer.SetArchiveBrowsing(true)
	assert.True(t, analyzer.archiveBrowsing)

	called := false
	filter := func(name string) bool { called = true; return false }
	analyzer.SetFileTypeFilter(filter)
	analyzer.ignoreFileType("test")
	assert.True(t, called)
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
	dir := analyzer.AnalyzeDir(
		tmpDir,
		func(name, _ string) bool { return name == "ignored" },
		func(_ string) bool { return false },
	)

	analyzer.GetDone().Wait()

	simpleDir := dir.(*SimpleDir)
	assert.Equal(t, 1, len(simpleDir.Files))
	assert.Equal(t, "top", simpleDir.Files[0].Name)
}

func TestTopDirGetFiles(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	)

	analyzer.GetDone().Wait()

	simpleDir := dir.(*SimpleDir)
	simpleDir.UpdateStats(make(fs.HardLinkedItems))

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
	analyzer.SetFollowSymlinks(true)
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SimpleDir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	var files []fs.Item
	for file := range dir.GetFiles(fs.SortBySize, fs.SortDesc) {
		files = append(files, file)
	}

	assert.Equal(t, int64(12+4096*4), dir.Size)
	assert.Equal(t, int64(6), dir.ItemCount)

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
	analyzer.SetFollowSymlinks(true)
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SimpleDir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	var files []fs.Item
	for file := range dir.GetFiles(fs.SortBySize, fs.SortDesc) {
		files = append(files, file)
	}

	assert.Equal(t, int64(12+4096*4), dir.Size)
	assert.Equal(t, int64(7), dir.ItemCount)
}
