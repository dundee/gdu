//go:build !windows && !linux

package remove

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenameNoReplacePropagatesLstatErrors(t *testing.T) {
	root := t.TempDir()
	loop := filepath.Join(root, "loop")
	require.NoError(t, os.Symlink("loop", loop))

	err := renameNoReplace(filepath.Join(root, "source"), filepath.Join(loop, "destination"))

	require.Error(t, err)
}
