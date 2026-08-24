package remove

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func mockTrashCommand(t *testing.T, run func(string, ...string) ([]byte, error)) {
	t.Helper()
	original := runTrashCommand
	t.Cleanup(func() { runTrashCommand = original })
	runTrashCommand = run
}

func sampleDir() (*analyze.Dir, *analyze.File) {
	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "test_dir",
			Size:  5,
			Usage: 12,
		},
		ItemCount: 2,
		BasePath:  ".",
	}
	file := &analyze.File{
		Name:   "file2",
		Size:   3,
		Usage:  4,
		Parent: dir,
	}
	dir.Files = fs.Files{file}
	return dir, file
}

func TestTrashByCommand(t *testing.T) {
	var gotName string
	var gotArgs []string
	mockTrashCommand(t, func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = args
		return nil, nil
	})

	dir, file := sampleDir()

	trasher := TrashByCommand([]string{"trash-put", "--trash-dir", "/tmp/custom"})
	err := trasher(dir, file)
	require.NoError(t, err)

	assert.Equal(t, "trash-put", gotName)
	assert.Equal(t, []string{"--trash-dir", "/tmp/custom", file.GetPath()}, gotArgs)
	assert.Equal(t, 0, len(dir.Files))
	assert.Equal(t, int64(1), dir.ItemCount)
}

func TestTrashByCommandSingleWord(t *testing.T) {
	var gotArgs []string
	mockTrashCommand(t, func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	})

	dir, file := sampleDir()

	err := TrashByCommand([]string{"trash-put"})(dir, file)
	require.NoError(t, err)
	assert.Equal(t, []string{file.GetPath()}, gotArgs)
}

func TestTrashByCommandEmpty(t *testing.T) {
	dir, file := sampleDir()

	err := TrashByCommand(nil)(dir, file)
	require.Error(t, err)
	assert.Equal(t, 1, len(dir.Files))
}

func TestTrashByCommandError(t *testing.T) {
	mockTrashCommand(t, func(name string, args ...string) ([]byte, error) {
		return []byte("trash-put: cannot trash\n"), fmt.Errorf("exit status 1")
	})

	dir, file := sampleDir()

	err := TrashByCommand([]string{"trash-put"})(dir, file)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 1")
	assert.Contains(t, err.Error(), "cannot trash")
	// The item stays in the tree when the command fails.
	assert.Equal(t, 1, len(dir.Files))
}
