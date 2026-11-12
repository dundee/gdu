//go:build !windows && !plan9

package remove

import (
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
