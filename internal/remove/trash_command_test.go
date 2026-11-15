//go:build !windows

package remove

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/fs"
)

func mockTrashCommandOS(t *testing.T, configure func(*trashCommandOSOps)) {
	t.Helper()
	original := trashCommandOS
	t.Cleanup(func() { trashCommandOS = original })
	configure(&trashCommandOS)
}

// createTestTree builds the in-memory counterpart of the directory created by
// testdir.CreateTestDir and returns its file2.
func createTestTree() *analyze.File {
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

	return file
}

func TestTrashCommand(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	file := createTestTree()

	moved, err := TrashCommand("rm -rf")(file)
	require.NoError(t, err)

	_, err = os.Stat("test_dir/nested/file2")
	assert.True(t, os.IsNotExist(err))

	assert.True(t, moved)
}

func TestTrashCommandOnDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

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
	dir.Files = fs.Files{subdir}

	moved, err := TrashCommand("rm -rf")(subdir)
	require.NoError(t, err)

	_, err = os.Stat("test_dir/nested")
	assert.True(t, os.IsNotExist(err))
	assert.True(t, moved)
}

func TestShellScript(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "path appended",
			command: "trash-put",
			want:    `trash-put "$@"`,
		},
		{
			name:    "path appended after arguments",
			command: "trash-put --trash-dir ~/mytrash",
			want:    `trash-put --trash-dir ~/mytrash "$@"`,
		},
		{
			name:    "env var is not a placeholder",
			command: "trash-put --trash-dir $HOME/mytrash",
			want:    `trash-put --trash-dir $HOME/mytrash "$@"`,
		},
		{
			name:    "quoted first parameter",
			command: `mv -f "$1" ~/mytrash/`,
			want:    `mv -f "$1" ~/mytrash/`,
		},
		{
			name:    "braced first parameter",
			command: `mv -f "${1}" ~/mytrash/`,
			want:    `mv -f "${1}" ~/mytrash/`,
		},
		{
			name:    "all parameters",
			command: `mv -f "$@" ~/mytrash/`,
			want:    `mv -f "$@" ~/mytrash/`,
		},
		{
			name:    "all parameters as single word",
			command: `mv -f "$*" ~/mytrash/`,
			want:    `mv -f "$*" ~/mytrash/`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shellScript(tt.command))
		})
	}
}

// writeScript creates a shell script with the given body and returns a command
// running it, so tests can use commands carrying their own arguments.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.sh")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return "/bin/sh " + path
}

// TestTrashCommandWithArguments checks that the path is appended to a command
// which already carries arguments of its own.
func TestTrashCommandWithArguments(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	target := t.TempDir()
	file := createTestTree()

	// $1 is the argument given in the command, $2 the appended item path.
	command := writeScript(t, `mv -f "$2" "$1"`) + " " + target

	moved, err := TrashCommand(command)(file)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(target, "file2"))
	assert.NoFileExists(t, "test_dir/nested/file2")
	assert.True(t, moved)
}

// TestTrashCommandWithMv covers the mv recipe from the documentation. mv takes
// its destination last, so the command places the item path itself instead of
// having it appended. It also checks that the command is evaluated by a shell,
// so a trash dir given in the config file can use ~.
func TestTrashCommandWithMv(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.Mkdir(filepath.Join(home, "mytrash"), 0o700))

	file := createTestTree()

	moved, err := TrashCommand(`mv -f "$1" ~/mytrash/`)(file)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(home, "mytrash", "file2"))
	assert.NoFileExists(t, "test_dir/nested/file2")
	assert.True(t, moved)
}

// TestTrashCommandWithMvToMissingDir makes sure the trailing slash in the
// documented mv recipe turns a missing trash dir into an error instead of
// renaming the item onto the destination path.
func TestTrashCommandWithMvToMissingDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	home := t.TempDir()
	t.Setenv("HOME", home)

	file := createTestTree()

	moved, err := TrashCommand(`mv -f "$1" ~/mytrash/`)(file)
	require.Error(t, err)

	assert.NoFileExists(t, filepath.Join(home, "mytrash"))
	assert.FileExists(t, "test_dir/nested/file2")
	assert.False(t, moved)
}

func TestTrashCommandPassesPathAsArgumentAndEnvVar(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out := filepath.Join(t.TempDir(), "args")
	file := createTestTree()

	command := writeScript(t, `printf '%s\n' "$#" "$1" "$GDU_TRASH_PATH" > "`+out+`"`)

	_, err := TrashCommand(command)(file)
	require.NoError(t, err)

	wantPath, err := filepath.Abs("test_dir/nested/file2")
	require.NoError(t, err)

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, []string{"1", wantPath, wantPath}, strings.Fields(string(data)))
}

// TestTrashCommandWithSpecialCharsInName makes sure item names are never
// interpreted as shell syntax.
func TestTrashCommandWithSpecialCharsInName(t *testing.T) {
	base := t.TempDir()
	name := "; touch pwned; echo 'x'"
	path := filepath.Join(base, name)
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))

	dir := &analyze.Dir{
		File:      &analyze.File{Name: filepath.Base(base)},
		ItemCount: 2,
		BasePath:  filepath.Dir(base),
	}
	file := &analyze.File{Name: name, Size: 1, Usage: 1, Parent: dir}
	dir.Files = fs.Files{file}

	moved, err := TrashCommand("rm -f")(file)
	require.NoError(t, err)

	assert.NoFileExists(t, path)
	assert.NoFileExists(t, filepath.Join(base, "pwned"))
	assert.NoFileExists(t, "pwned")
	assert.True(t, moved)
}

func TestTrashCommandFailing(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	file := createTestTree()

	moved, err := TrashCommand(writeScript(t, "exit 3"))(file)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running trash command")
	assert.Contains(t, err.Error(), "exit status 3")

	assert.FileExists(t, "test_dir/nested/file2")
	assert.False(t, moved)
}

func TestTrashCommandFailingWithStderr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	file := createTestTree()

	moved, err := TrashCommand(writeScript(t, "echo 'trash dir is missing' >&2\nexit 1"))(file)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trash dir is missing")
	assert.False(t, moved)
}

// TestTrashCommandLeavingItemInPlace covers a command which succeeds without
// removing the item, e.g. an interactive command answered with "no".
func TestTrashCommandLeavingItemInPlace(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	file := createTestTree()

	moved, err := TrashCommand("true")(file)
	require.NoError(t, err)

	assert.FileExists(t, "test_dir/nested/file2")
	assert.False(t, moved)
}

func TestTrashCommandAbsError(t *testing.T) {
	sentinel := errors.New("abs failed")
	mockTrashCommandOS(t, func(ops *trashCommandOSOps) {
		ops.abs = func(string) (string, error) { return "", sentinel }
	})

	file := createTestTree()

	moved, err := TrashCommand("rm -rf")(file)
	require.ErrorIs(t, err, sentinel)
	assert.False(t, moved)
}

func TestTrashCommandStderrTruncated(t *testing.T) {
	mockTrashCommandOS(t, func(ops *trashCommandOSOps) {
		ops.run = func(_ string, _, _ []string, stderr io.Writer) error {
			_, err := stderr.Write([]byte(strings.Repeat("x", maxStderrLen+10)))
			require.NoError(t, err)
			return errors.New("failed")
		}
	})

	file := createTestTree()

	_, err := TrashCommand("rm -rf")(file)
	require.Error(t, err)
	assert.True(t, strings.HasSuffix(err.Error(), "..."))
	assert.Less(t, len(err.Error()), maxStderrLen+100)
}
