package tui

import (
	"bytes"
	"testing"
	"time"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/testanalyze"
	"github.com/dundee/gdu/v6/internal/testapp"
	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestDoubleClick(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})
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

func TestDoubleClickNoViewFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(2, 0)
	ui.setNoViewFile()
	previousHeaderText := ui.header.GetText(false)

	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseLeftDoubleClick)
	assert.False(t, ui.pages.HasPage("file"))
	assert.Equal(t, " Viewing files is disabled!", ui.header.GetText(false))

	time.Sleep(2100 * time.Millisecond)
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, previousHeaderText, ui.header.GetText(false))
}

func TestScroll(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})
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
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})
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
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})

	event, action := ui.onMouse(nil, tview.MouseMove)
	assert.True(t, event == nil)
	assert.Equal(t, action, tview.MouseMove)
}

func TestMouseMove(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})

	event, action := ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseMove)
	assert.True(t, event != nil)
	assert.Equal(t, action, tview.MouseMove)
}

// A double-click in the list of devices chooses the device; it used to panic.
func TestDoubleClickChoosesDevice(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	assert.NoError(t, ui.ListDevices(getDevicesInfoMock()))
	ui.table.Select(1, 0)

	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseLeftDoubleClick)

	<-ui.done
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}
	assert.Equal(t, "test_dir", ui.currentDir.GetName())
}

// A double-click in an empty directory does nothing; it used to panic.
func TestDoubleClickInEmptyDirectory(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})
	ui.done = make(chan struct{})
	assert.NoError(t, ui.AnalyzePath(t.TempDir(), nil))
	<-ui.done
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NotPanics(t, func() {
		ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseLeftDoubleClick)
	})
}

// While the file viewer is open, the mouse does not reach the listing behind
// it, which used to open another viewer.
func TestMouseIgnoredUnderFileViewer(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})
	ui.done = make(chan struct{})
	assert.NoError(t, ui.AnalyzePath("test_dir/nested", nil))
	<-ui.done
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}
	for row := range ui.table.GetRowCount() {
		if item, ok := ui.table.GetCell(row, 0).GetReference().(fs.Item); ok && item.GetName() == "file2" {
			ui.table.Select(row, 0)
		}
	}
	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseLeftDoubleClick)
	assert.True(t, ui.pages.HasPage("file"))
	row, _ := ui.table.GetSelection()

	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseScrollUp)
	ui.onMouse(tcell.NewEventMouse(0, 0, 0, 0), tview.MouseLeftDoubleClick)

	after, _ := ui.table.GetSelection()
	assert.Equal(t, row, after)
}
