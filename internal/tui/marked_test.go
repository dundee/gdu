package tui

import (
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestItemMarked(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	ui.fileItemMarked(0)
	item, ok := ui.rowItem(0, 0)
	assert.True(t, ok)
	assert.Equal(t, map[fs.Item]struct{}{item: {}}, ui.markedItems)

	ui.fileItemMarked(0)
	assert.Equal(t, map[fs.Item]struct{}{}, ui.markedItems)
}
