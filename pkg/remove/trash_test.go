//go:build !windows

package remove

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func TestMoveItemToTrash(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "test_dir",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  ".",
	}
	subdir := &analyze.Dir{
		File: &analyze.File{
			Name:   "nested",
			Size:   4,
			Usage:  8,
			Parent: dir,
		},
		ItemCount: 2,
	}
	file := &analyze.File{
		Name:   "file2",
		Size:   3,
		Usage:  4,
		Parent: subdir,
	}
	dir.Files = fs.Files{subdir}
	subdir.Files = fs.Files{file}

	err := MoveItemToTrash(subdir, file)
	require.NoError(t, err)

	_, err = os.Stat("test_dir/nested/file2")
	assert.True(t, os.IsNotExist(err))

	trashFile := filepath.Join(xdg, "Trash", "files", "file2")
	_, err = os.Stat(trashFile)
	assert.NoError(t, err)

	infoPath := filepath.Join(xdg, "Trash", "info", "file2.trashinfo")
	data, err := os.ReadFile(infoPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "[Trash Info]")
	assert.Contains(t, string(data), "Path=")
	assert.Contains(t, string(data), "DeletionDate=")
	assert.True(t, strings.Contains(string(data), "file2") || strings.Contains(string(data), "nested"))

	assert.Equal(t, 0, len(subdir.Files))
	assert.Equal(t, int64(1), subdir.ItemCount)
}

func TestMoveItemToTrashNameConflict(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	trashFiles := filepath.Join(xdg, "Trash", "files")
	require.NoError(t, os.MkdirAll(trashFiles, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(trashFiles, "file2"), []byte("old"), 0o600))

	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "test_dir",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 3,
		BasePath:  ".",
	}
	subdir := &analyze.Dir{
		File: &analyze.File{
			Name:   "nested",
			Size:   4,
			Usage:  8,
			Parent: dir,
		},
		ItemCount: 2,
	}
	file := &analyze.File{
		Name:   "file2",
		Size:   3,
		Usage:  4,
		Parent: subdir,
	}
	dir.Files = fs.Files{subdir}
	subdir.Files = fs.Files{file}

	err := MoveItemToTrash(subdir, file)
	require.NoError(t, err)

	_, err = os.Stat("test_dir/nested/file2")
	assert.True(t, os.IsNotExist(err))

	oldData, err := os.ReadFile(filepath.Join(trashFiles, "file2"))
	require.NoError(t, err)
	assert.Equal(t, "old", string(oldData))

	movedData, err := os.ReadFile(filepath.Join(trashFiles, "file2.2"))
	require.NoError(t, err)
	assert.Equal(t, "go", string(movedData))

	infoPath := filepath.Join(xdg, "Trash", "info", "file2.2.trashinfo")
	_, err = os.Stat(infoPath)
	assert.NoError(t, err)

	assert.Equal(t, 0, len(subdir.Files))
	assert.Equal(t, int64(1), subdir.ItemCount)
}
