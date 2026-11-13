//go:build !windows && !plan9

package remove

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Opening a FIFO for writing blocks until a reader shows up, so emptying one
// must be refused before it is opened.
func TestDeleterEmptyRefusesFIFO(t *testing.T) {
	root := makeScanRoot(t)
	require.NoError(t, syscall.Mkfifo(filepath.Join(root, "fifo"), 0o600))
	tree := scanTree(t, root, false)

	err := NewDeleter([]string{root}).Delete(tree, child(t, tree, "fifo"), ModeEmpty)

	assert.ErrorIs(t, err, ErrNotRegular)
}

// lockDir makes path unreadable until the test ends.
func lockDir(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permissions do not apply to root")
	}
	require.NoError(t, os.Chmod(path, 0))
	t.Cleanup(func() { assert.NoError(t, os.Chmod(path, 0o755)) })
}

// A removal the filesystem refuses is reported as it is, and the item stays
// in the tree.
func TestDeleterReportsPermissionErrors(t *testing.T) {
	for _, locked := range []string{"dir", filepath.Join("dir", "b")} {
		for _, parallel := range []bool{false, true} {
			root := makeScanRoot(t)
			tree := scanTree(t, root, false)
			lockDir(t, filepath.Join(root, locked))
			var opts []Option
			if parallel {
				opts = append(opts, WithParallel())
			}

			err := NewDeleter([]string{root}, opts...).Delete(tree, child(t, tree, "dir"), ModeRemove)

			assert.ErrorIs(t, err, os.ErrPermission, "locked %s, parallel %v", locked, parallel)
			assert.False(t, errors.Is(err, ErrRefused))
			assert.True(t, hasChild(tree, "dir"))
		}
	}
}
