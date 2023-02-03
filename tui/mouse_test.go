package tui

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestDoubleClick(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	ui.table.Select(0, 0)
	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseLeftDoubleClick)
	assert.Equal(t, "nested", ui.currentDir.GetName())

	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseLeftDoubleClick)
	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	// show file content
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(2, 0)
	selectedFile := ui.table.GetCell(2, 0).GetReference().(fs.Item)
	assert.Equal(t, selectedFile.GetName(), "file2")
	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseLeftDoubleClick)
	assert.True(t, ui.pages.HasPage("file"))
}

func TestScroll(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseScrollDown)
	row, _ := ui.table.GetSelection()
	assert.Equal(t, row, 1)

	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseScrollUp)
	row, _ = ui.table.GetSelection()
	assert.Equal(t, row, 0)
}

func TestScrollWhenPageOpened(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	// open confirm dialog
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseScrollDown)
	row, _ := ui.table.GetSelection()
	// scrolling does nothing
	assert.Equal(t, 0, row)
}

func TestEmptyEvent(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	event, action := ui.onMouse(nil, tview.MouseMove)
	assert.True(t, event == nil)
	assert.Equal(t, action, tview.MouseMove)
}

func TestMouseMove(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	event, action := ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseMove)
	assert.True(t, event != nil)
	assert.Equal(t, action, tview.MouseMove)
}
