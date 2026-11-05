//go:build linux

package analyze

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNestedBtrfsSubvolumeFalseForPlainDir(t *testing.T) {
	dir := t.TempDir()
	info, err := os.Lstat(dir)
	require.NoError(t, err)

	assert.False(t, nestedBtrfsSubvolume(dir, info, map[string]struct{}{}))
}
