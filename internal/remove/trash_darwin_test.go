//go:build darwin

package remove

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/pkg/analyze"
)

func darwinTrashFixture(t *testing.T) (*analyze.File, string) {
	t.Helper()
	base := t.TempDir()
	path := filepath.Join(base, "-rf file")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))

	dir := &analyze.Dir{File: &analyze.File{Name: filepath.Base(base)}, BasePath: filepath.Dir(base)}
	item := &analyze.File{Name: "-rf file", Parent: dir}
	dir.Files = append(dir.Files, item)
	return item, path
}

func TestMoveItemToTrashRunsSystemTrash(t *testing.T) {
	item, path := darwinTrashFixture(t)

	var gotArgv0 string
	var gotArgv []string
	orig := trashCommandOS.run
	trashCommandOS.run = func(argv0 string, argv, _ []string, _ io.Writer) error {
		gotArgv0, gotArgv = argv0, argv
		return os.Remove(path)
	}
	defer func() { trashCommandOS.run = orig }()

	moved, err := MoveItemToTrash(item)
	require.NoError(t, err)
	assert.Equal(t, macOSTrashBin, gotArgv0)
	assert.Equal(t, []string{path}, gotArgv)
	assert.True(t, moved)
}

func TestMoveItemToTrashKeepsItemThatIsStillThere(t *testing.T) {
	item, _ := darwinTrashFixture(t)

	orig := trashCommandOS.run
	trashCommandOS.run = func(string, []string, []string, io.Writer) error { return nil }
	defer func() { trashCommandOS.run = orig }()

	moved, err := MoveItemToTrash(item)
	require.NoError(t, err)
	assert.False(t, moved)
}

func TestMoveItemToTrashMissingBinary(t *testing.T) {
	item, _ := darwinTrashFixture(t)

	orig := macOSTrashBin
	macOSTrashBin = filepath.Join(t.TempDir(), "no-trash")
	defer func() { macOSTrashBin = orig }()

	moved, err := MoveItemToTrash(item)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trash-command")
	assert.False(t, moved)
}

// TestMoveItemToTrashWithSystemTrash runs the real /usr/bin/trash, which puts
// the test file into the Trash of whoever runs it, so it only runs when
// GDU_TEST_SYSTEM_TRASH=1 is set (as in CI).
func TestMoveItemToTrashWithSystemTrash(t *testing.T) {
	if os.Getenv("GDU_TEST_SYSTEM_TRASH") != "1" {
		t.Skip("set GDU_TEST_SYSTEM_TRASH=1 to move a test file into the real Trash")
	}
	if _, err := os.Stat(macOSTrashBin); err != nil {
		t.Skipf("%s is not available: %s", macOSTrashBin, err)
	}
	item, path := darwinTrashFixture(t)

	moved, err := MoveItemToTrash(item)
	require.NoError(t, err)
	_, err = os.Lstat(path)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.True(t, moved)
}
