package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeleteActionVerbAndActing(t *testing.T) {
	assert.Equal(t, "delete", ActionDelete.Verb())
	assert.Equal(t, "empty", ActionEmpty.Verb())
	assert.Equal(t, "move to trash", ActionMoveToTrash.Verb())
	assert.Equal(t, "delete", DeleteAction(99).Verb())

	assert.Equal(t, "deleting", ActionDelete.Acting())
	assert.Equal(t, "emptying", ActionEmpty.Acting())
	assert.Equal(t, "moving to trash", ActionMoveToTrash.Acting())
	assert.Equal(t, "deleting", DeleteAction(99).Acting())
}
