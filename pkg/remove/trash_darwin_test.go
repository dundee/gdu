//go:build darwin

package remove

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoveItemToTrashUnsupportedOnDarwin(t *testing.T) {
	err := MoveItemToTrash(nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported on macOS")
}
