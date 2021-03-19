package tui

import (
	"runtime"
	"testing"

	"github.com/dundee/gdu/v4/analyze"
	"github.com/dundee/gdu/v4/device"
	"github.com/dundee/gdu/v4/internal/testanalyze"
	"github.com/dundee/gdu/v4/internal/testapp"
	"github.com/dundee/gdu/v4/internal/testdir"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestShowDevices(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, true, true)
	ui.ListDevices(getDevicesInfoMock())
	ui.table.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte("Device name")
	for i, r := range b[0:11] {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestShowDevicesBW(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, false, false)
	ui.ListDevices(getDevicesInfoMock())
	ui.table.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte("Device name")
	for i, r := range b[0:11] {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestShowDevicesWithError(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	getter := device.LinuxDevicesInfoGetter{MountsPath: "/xyzxyz"}

	ui := CreateUI(app, false, false)
	err := ui.ListDevices(getter)

	assert.Contains(t, err.Error(), "no such file")
}

func TestDeviceSelected(t *testing.T) {
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)
	ui.analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.ListDevices(getDevicesInfoMock())

	assert.Equal(t, 3, ui.table.GetRowCount())

	ui.deviceItemSelected(1, 0)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
}

func TestAnalyzePath(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
}

func TestAnalyzePathWithErr(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	err := ui.AnalyzePath("xxx", nil)

	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestAnalyzePathBW(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
}

func TestAnalyzePathWithParentDir(t *testing.T) {
	parentDir := &analyze.Dir{
		File: &analyze.File{
			Name: "parent",
		},
		Files: make([]analyze.Item, 0, 1),
	}

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = &testanalyze.MockedAnalyzer{}
	ui.pathChecker = testdir.MockedPathChecker
	ui.topDir = parentDir
	ui.done = make(chan struct{})
	ui.AnalyzePath("test_dir", parentDir)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)
	assert.Equal(t, parentDir, ui.currentDir.Parent)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
}

func TestViewDirContents(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.analyzer = &testanalyze.MockedAnalyzer{}
	ui.pathChecker = testdir.MockedPathChecker
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
	ui.pathChecker = testdir.MockedPathChecker
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
