package tui

import (
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
)

func TestItemMarked(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	ui.fileItemMarked(1)
	assert.Equal(t, ui.markedRows, map[int]struct{}{1: {}})

	ui.fileItemMarked(1)
	assert.Equal(t, ui.markedRows, map[int]struct{}{})
}
