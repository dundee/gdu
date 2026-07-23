//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package remove

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyRecursivelyRejectsFIFOWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "fifo")
	dst := filepath.Join(root, "copied-fifo")
	require.NoError(t, syscall.Mkfifo(src, 0o600))

	done := make(chan error, 1)
	go func() {
		done <- copyRecursively(src, dst)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file type")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("copyRecursively blocked while opening a FIFO")
	}
}
