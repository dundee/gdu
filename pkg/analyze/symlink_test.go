package analyze

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestReadSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	linkPath := filepath.Join(dir, "link")
	err := os.Symlink("target/path", linkPath)
	assert.Nil(t, err)

	info, err := os.Lstat(linkPath)
	assert.Nil(t, err)

	assert.Equal(t, "target/path", readSymlinkTarget(info.Mode(), linkPath))
}

func TestReadSymlinkTargetNonSymlink(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file")
	err := os.WriteFile(filePath, []byte("hi"), 0o600)
	assert.Nil(t, err)

	info, err := os.Lstat(filePath)
	assert.Nil(t, err)

	assert.Equal(t, "", readSymlinkTarget(info.Mode(), filePath))
}

func TestReadSymlinkTargetUnreadable(t *testing.T) {
	// A path flagged as a symlink but with no readable link returns empty.
	assert.Equal(t, "", readSymlinkTarget(os.ModeSymlink, "nonexistent/link"))
}

// findSymlink locates a file item named "link" anywhere in the analyzed tree.
func findSymlink(item fs.Item) fs.Item {
	if !item.IsDir() && item.GetName() == "link" {
		return item
	}
	for child := range item.GetFiles(fs.SortByName, fs.SortAsc) {
		if found := findSymlink(child); found != nil {
			return found
		}
	}
	return nil
}

// analyzerFactory builds a fresh analyzer that returns the full tree.
type analyzerFactory struct {
	name    string
	analyze func(path string) fs.Item
}

func symlinkTreeAnalyzers() []analyzerFactory {
	ignoreDir := func(_, _ string) bool { return false }
	ignoreType := func(_ string) bool { return false }
	return []analyzerFactory{
		{"parallel", func(path string) fs.Item {
			a := CreateAnalyzer()
			item := a.AnalyzeDir(path, ignoreDir, ignoreType)
			a.GetDone().Wait()
			return item
		}},
		{"sequential", func(path string) fs.Item {
			a := CreateSeqAnalyzer()
			item := a.AnalyzeDir(path, ignoreDir, ignoreType)
			a.GetDone().Wait()
			return item
		}},
		{"stable", func(path string) fs.Item {
			a := CreateStableOrderAnalyzer()
			item := a.AnalyzeDir(path, ignoreDir, ignoreType)
			a.GetDone().Wait()
			return item
		}},
	}
}

// TestAnalyzersPopulateSymlinkTarget guards against the shotgun-surgery hazard:
// every tree-returning analyzer must set the symlink target on File items.
func TestAnalyzersPopulateSymlinkTarget(t *testing.T) {
	for _, tc := range symlinkTreeAnalyzers() {
		t.Run(tc.name, func(t *testing.T) {
			fin := testdir.CreateTestDir()
			defer fin()

			err := os.Symlink("nested/file2", "test_dir/link")
			assert.Nil(t, err)

			tree := tc.analyze("test_dir")
			link := findSymlink(tree)
			assert.NotNil(t, link, "symlink item should be present in the tree")

			si, ok := link.(fs.SymlinkItem)
			assert.True(t, ok, "File must implement fs.SymlinkItem")
			assert.Equal(t, "nested/file2", si.GetSymlinkTarget())
			assert.Equal(t, "Symlink", link.GetType())
		})
	}
}

// TestTopDirAnalyzerPopulatesSymlinkTarget covers the non-interactive top-dir
// analyzer, which returns SimpleDir items reconstructed on GetFiles.
func TestTopDirAnalyzerPopulatesSymlinkTarget(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Symlink("nested/file2", "test_dir/link")
	assert.Nil(t, err)

	analyzer := CreateTopDirAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*SimpleDir)
	analyzer.GetDone().Wait()

	link := findSymlink(dir)
	assert.NotNil(t, link, "symlink item should be present in the tree")
	si, ok := link.(fs.SymlinkItem)
	assert.True(t, ok, "File must implement fs.SymlinkItem")
	assert.Equal(t, "nested/file2", si.GetSymlinkTarget())
}

// TestStoredAnalyzerPopulatesSymlinkTarget covers the stored analyzer, whose
// items are scanned into the top-level dir before being persisted.
func TestStoredAnalyzerPopulatesSymlinkTarget(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Symlink("nested/file2", "test_dir/link")
	assert.Nil(t, err)

	a := CreateStoredAnalyzer("/tmp/badger")
	dir := a.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, func(_ string) bool { return false },
	).(*StoredDir)
	a.GetDone().Wait()

	idx, ok := dir.Files.FindByName("link")
	assert.True(t, ok, "symlink item should be present in the tree")
	si, ok := dir.Files[idx].(fs.SymlinkItem)
	assert.True(t, ok, "File must implement fs.SymlinkItem")
	assert.Equal(t, "nested/file2", si.GetSymlinkTarget())
}

func TestFollowSymlinkErr(t *testing.T) {
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

	err = os.Symlink(
		"test_dir/nested",
		"test_dir/some_dir")
	assert.Nil(t, err)

	_, err = followSymlink("xxx", false)
	assert.ErrorContains(t, err, "no such file or directory")

	_, err = followSymlink("test_dir/nested/file3", false)
	assert.ErrorContains(t, err, "no such file or directory")

	_, err = followSymlink("test_dir/nested/file3", true)
	assert.NoError(t, err)

	res, err := followSymlink("test_dir/some_dir", true)
	assert.Equal(t, nil, res)
	assert.NoError(t, err)
}
