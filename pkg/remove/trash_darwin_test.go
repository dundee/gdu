//go:build darwin

package remove

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

func darwinTrashFixture(t *testing.T) (*analyze.Dir, *analyze.File, string) {
	t.Helper()
	base := t.TempDir()
	path := filepath.Join(base, "-rf file")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))

	dir := &analyze.Dir{File: &analyze.File{Name: filepath.Base(base)}, BasePath: filepath.Dir(base)}
	item := &analyze.File{Name: "-rf file", Parent: dir}
	dir.Files = append(dir.Files, item)
	return dir, item, path
}

func TestMoveItemToTrashRunsSystemTrash(t *testing.T) {
	dir, item, path := darwinTrashFixture(t)

	var gotArgv0 string
	var gotArgv []string
	orig := trashCommandOS.run
	trashCommandOS.run = func(argv0 string, argv, _ []string, _ io.Writer) error {
		gotArgv0, gotArgv = argv0, argv
		return os.Remove(path)
	}
	defer func() { trashCommandOS.run = orig }()

	require.NoError(t, MoveItemToTrash(dir, item))
	assert.Equal(t, macOSTrashBin, gotArgv0)
	assert.Equal(t, []string{path}, gotArgv)
	assert.Empty(t, dir.Files)
}

func TestMoveItemToTrashKeepsItemThatIsStillThere(t *testing.T) {
	dir, item, _ := darwinTrashFixture(t)

	orig := trashCommandOS.run
	trashCommandOS.run = func(string, []string, []string, io.Writer) error { return nil }
	defer func() { trashCommandOS.run = orig }()

	require.NoError(t, MoveItemToTrash(dir, item))
	assert.Len(t, dir.Files, 1)
}

func TestMoveItemToTrashMissingBinary(t *testing.T) {
	dir, item, _ := darwinTrashFixture(t)

	orig := macOSTrashBin
	macOSTrashBin = filepath.Join(t.TempDir(), "no-trash")
	defer func() { macOSTrashBin = orig }()

	err := MoveItemToTrash(dir, item)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trash-command")
	assert.Len(t, dir.Files, 1)
}
