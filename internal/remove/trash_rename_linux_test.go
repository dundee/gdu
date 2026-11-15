//go:build linux

package remove

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// withRenameat2Failing makes renameat2 fail with errno, as it does on a
// filesystem or kernel without RENAME_NOREPLACE.
func withRenameat2Failing(t *testing.T, errno error) {
	t.Helper()
	orig := renameat2
	t.Cleanup(func() { renameat2 = orig })
	renameat2 = func(int, string, int, string, uint) error { return errno }
}

// On NFS, FUSE servers without rename2 and older ZFS, renameat2 refuses the
// flag; trashing used to fail there with "invalid argument".
func TestRenameNoReplaceFallsBackWithoutTheFlag(t *testing.T) {
	for _, errno := range []error{unix.EINVAL, unix.ENOSYS, unix.EOPNOTSUPP} {
		dir := t.TempDir()
		src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
		require.NoError(t, os.WriteFile(src, []byte("x"), 0o600))
		withRenameat2Failing(t, errno)

		require.NoError(t, renameNoReplace(src, dst), errno)

		assert.FileExists(t, dst)
		assert.NoFileExists(t, src)
	}
}

func TestRenameNoReplaceFallbackKeepsAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o600))
	withRenameat2Failing(t, unix.EINVAL)

	assert.ErrorIs(t, renameNoReplace(src, dst), os.ErrExist)

	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "old", string(content))
}

// Other errors name the paths, and EXDEV still reads as one, so a move
// across filesystems copies.
func TestRenameNoReplaceErrorsNameThePaths(t *testing.T) {
	withRenameat2Failing(t, unix.EXDEV)

	err := renameNoReplace("/a/src", "/b/dst")

	assert.ErrorContains(t, err, "/a/src")
	assert.ErrorContains(t, err, "/b/dst")
	assert.True(t, isEXDEV(err))
}

func TestRenameNoReplace(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "taken"), []byte("y"), 0o600))

	assert.ErrorIs(t, renameNoReplace(src, filepath.Join(dir, "taken")), os.ErrExist)
	require.NoError(t, renameNoReplace(src, dst))
	assert.FileExists(t, dst)
}
