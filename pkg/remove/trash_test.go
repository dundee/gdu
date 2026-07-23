//go:build !windows

package remove

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
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
	assert.Contains(t, string(data), "DeletionDate=")
	wantPath, err := filepath.Abs("test_dir/nested/file2")
	require.NoError(t, err)
	assert.Contains(t, string(data), "Path="+escapeTrashPath(wantPath))

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

func TestReserveTrashInfoUsesUniqueNamesConcurrently(t *testing.T) {
	trashRoot := t.TempDir()
	filesDir := filepath.Join(trashRoot, "files")
	infoDir := filepath.Join(trashRoot, "info")
	require.NoError(t, os.MkdirAll(filesDir, 0o700))
	require.NoError(t, os.MkdirAll(infoDir, 0o700))

	const workers = 16
	start := make(chan struct{})
	names := make(chan string, workers)
	errs := make(chan error, workers)

	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			name, _, err := reserveTrashInfo(
				filesDir,
				infoDir,
				"same-name",
				filepath.Join("/source", fmt.Sprintf("%d", i), "same-name"),
			)
			if err != nil {
				errs <- err
				return
			}
			names <- name
		}()
	}

	close(start)
	wg.Wait()
	close(names)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	uniqueNames := make(map[string]struct{}, workers)
	for name := range names {
		uniqueNames[name] = struct{}{}
	}
	assert.Len(t, uniqueNames, workers)

	entries, err := os.ReadDir(infoDir)
	require.NoError(t, err)
	assert.Len(t, entries, workers)
}

func TestCopyRecursivelyPreservesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	src := filepath.Join(root, "link")
	dst := filepath.Join(root, "copied-link")

	require.NoError(t, os.WriteFile(target, []byte("target contents"), 0o600))
	require.NoError(t, os.Symlink("target", src))

	require.NoError(t, copyRecursively(src, dst))

	info, err := os.Lstat(dst)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)

	linkTarget, err := os.Readlink(dst)
	require.NoError(t, err)
	assert.Equal(t, "target", linkTarget)
}

func TestMovePathDoesNotOverwriteExisting(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o600))

	err := movePath(src, dst)
	require.Error(t, err)
	assert.True(t, os.IsExist(err))

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "old", string(data))

	_, err = os.Stat(src)
	assert.NoError(t, err)
}

func TestTrashDirUsesHomeWhenXDGUnset(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := trashDir()
	require.NoError(t, err)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".local", "share", "Trash"), dir)
}

func TestEscapeTrashPathPercentEncodesUnsafeBytes(t *testing.T) {
	got := escapeTrashPath("/tmp/has space/%and\n")
	assert.Equal(t, "/tmp/has%20space/%25and%0A", got)
}

func TestIsEXDEV(t *testing.T) {
	assert.True(t, isEXDEV(syscall.EXDEV))
	assert.True(t, isEXDEV(&os.PathError{Op: "rename", Err: syscall.EXDEV}))
	assert.False(t, isEXDEV(os.ErrExist))
}

func TestCopyRecursivelyCopiesRegularFileAndDirectory(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "src")
	dstDir := filepath.Join(root, "dst")
	require.NoError(t, os.Mkdir(srcDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(srcDir, "sub"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("world"), 0o600))

	require.NoError(t, copyRecursively(srcDir, dstDir))

	data, err := os.ReadFile(filepath.Join(dstDir, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	data, err = os.ReadFile(filepath.Join(dstDir, "sub", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "world", string(data))
}

func TestCopyRecursivelyMissingSource(t *testing.T) {
	err := copyRecursively(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "dst"))
	require.Error(t, err)
}

func TestMovePathFallsBackOnEXDEV(t *testing.T) {
	orig := renameNoReplaceFn
	t.Cleanup(func() { renameNoReplaceFn = orig })
	renameNoReplaceFn = func(oldpath, newpath string) error {
		return &os.PathError{Op: "rename", Path: oldpath, Err: syscall.EXDEV}
	}

	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	require.NoError(t, os.WriteFile(src, []byte("payload"), 0o600))

	require.NoError(t, movePath(src, dst))

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "payload", string(data))
	_, err = os.Stat(src)
	assert.True(t, os.IsNotExist(err))
}

func TestMovePathEXDEVCleanupOnCopyFailure(t *testing.T) {
	orig := renameNoReplaceFn
	t.Cleanup(func() { renameNoReplaceFn = orig })
	renameNoReplaceFn = func(oldpath, newpath string) error {
		return &os.PathError{Op: "rename", Path: oldpath, Err: syscall.EXDEV}
	}

	root := t.TempDir()
	src := filepath.Join(root, "missing-src")
	dst := filepath.Join(root, "dst")

	err := movePath(src, dst)
	require.Error(t, err)
	_, err = os.Stat(dst)
	assert.True(t, os.IsNotExist(err))
}

func TestMoveItemToTrashRetriesWhenDestinationAppears(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	orig := renameNoReplaceFn
	t.Cleanup(func() { renameNoReplaceFn = orig })
	attempts := 0
	renameNoReplaceFn = func(oldpath, newpath string) error {
		attempts++
		if attempts == 1 {
			return os.ErrExist
		}
		return renameNoReplace(oldpath, newpath)
	}

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

	require.NoError(t, MoveItemToTrash(subdir, file))
	assert.GreaterOrEqual(t, attempts, 2)

	_, err := os.Stat("test_dir/nested/file2")
	assert.True(t, os.IsNotExist(err))
	assert.Equal(t, 0, len(subdir.Files))
}

func TestReserveTrashInfoSkipsExistingTrashinfo(t *testing.T) {
	trashRoot := t.TempDir()
	filesDir := filepath.Join(trashRoot, "files")
	infoDir := filepath.Join(trashRoot, "info")
	require.NoError(t, os.MkdirAll(filesDir, 0o700))
	require.NoError(t, os.MkdirAll(infoDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(infoDir, "item.trashinfo"), []byte("stale"), 0o600))

	name, infoPath, err := reserveTrashInfo(filesDir, infoDir, "item", "/tmp/item")
	require.NoError(t, err)
	assert.Equal(t, "item.2", name)
	assert.Equal(t, filepath.Join(infoDir, "item.2.trashinfo"), infoPath)
}

func TestMoveItemToTrashPropagatesMoveErrors(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	orig := renameNoReplaceFn
	t.Cleanup(func() { renameNoReplaceFn = orig })
	renameNoReplaceFn = func(oldpath, newpath string) error {
		return os.ErrPermission
	}

	dir := &analyze.Dir{
		File:      &analyze.File{Name: "test_dir", Size: 5, Usage: 12},
		ItemCount: 3,
		BasePath:  ".",
	}
	subdir := &analyze.Dir{
		File:      &analyze.File{Name: "nested", Size: 4, Usage: 8, Parent: dir},
		ItemCount: 2,
	}
	file := &analyze.File{Name: "file2", Size: 3, Usage: 4, Parent: subdir}
	dir.Files = fs.Files{subdir}
	subdir.Files = fs.Files{file}

	err := MoveItemToTrash(subdir, file)
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrPermission)
	assert.FileExists(t, "test_dir/nested/file2")
	assert.Len(t, subdir.Files, 1)
}

func TestCopyRecursivelyRejectsExistingDestination(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o600))

	err := copyRecursively(src, dst)
	require.Error(t, err)
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "old", string(data))
}

func TestCopyRecursivelyMkdirFailsWhenDestinationIsFile(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	require.NoError(t, os.Mkdir(src, 0o700))
	require.NoError(t, os.WriteFile(dst, []byte("file"), 0o600))

	err := copyRecursively(src, dst)
	require.Error(t, err)
}
