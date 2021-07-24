package tui

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestShowDevices(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, true, true)
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)

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
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)

	ui.table.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte("Device name")
	for i, r := range b[0:11] {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestDeviceSelected(t *testing.T) {
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)
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

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa")
}

func TestAnalyzePathBW(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa")
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
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.topDir = parentDir
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", parentDir)
	assert.Nil(t, err)

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

func TestReadAnalysis(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/test.json", os.O_RDONLY, 0644)
	assert.Nil(t, err)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, true, true)
	ui.done = make(chan struct{})

	err = ui.ReadAnalysis(input)
	assert.Nil(t, err)

	<-ui.done // wait for reading

	assert.Equal(t, "gdu", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}
}

func TestReadAnalysisWithWrongFile(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/wrong.json", os.O_RDONLY, 0644)
	assert.Nil(t, err)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, true, true)
	ui.done = make(chan struct{})

	err = ui.ReadAnalysis(input)
	assert.Nil(t, err)

	<-ui.done // wait for reading

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.True(t, ui.pages.HasPage("error"))
}

func TestViewDirContents(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

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
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.table.Select(3, 0)

	selectedFile := ui.table.GetCell(3, 0).GetReference().(analyze.Item)
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
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

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

func TestShowInfo(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))
	ui.table.Select(2, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))

	assert.True(t, ui.pages.HasPage("info"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0))

	assert.False(t, ui.pages.HasPage("info"))
}

func TestShowInfoBW(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, true, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))
	ui.table.Select(2, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))

	assert.True(t, ui.pages.HasPage("info"))
}

func TestExitViewFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

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
