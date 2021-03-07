package tui

import (
	"testing"

	"github.com/dundee/gdu/v4/analyze"
	"github.com/dundee/gdu/v4/internal/testanalyze"
	"github.com/dundee/gdu/v4/internal/testapp"
	"github.com/dundee/gdu/v4/internal/testdir"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestViewDirContents(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.AnalyzePath("test_dir", nil)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	res := ui.showFile() // selected item is dir, do nothing
	assert.Nil(t, res)
}

func TestViewContentsOfNotExistingFile(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.AnalyzePath("test_dir", nil)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	ui.table.Select(4, 0)

	selectedFile := ui.table.GetCell(4, 0).GetReference().(analyze.Item)
	assert.Equal(t, "ddd", selectedFile.GetName())

	res := ui.showFile()
	assert.Nil(t, res)
}

func TestViewFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	ui.AnalyzePath("test_dir", nil)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(2, 0)

	file := ui.showFile()
	assert.True(t, ui.pages.HasPage("file"))

	event := file.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'j', 0))
	assert.Equal(t, 'j', event.Rune())
}

func TestExitViewFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	ui.AnalyzePath("test_dir", nil)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(2, 0)

	file := ui.showFile()

	assert.True(t, ui.pages.HasPage("file"))

	file.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'q', 0))

	assert.False(t, ui.pages.HasPage("file"))
}
