package tui

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestShowDevices(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
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

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false, false)
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
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, true, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)
	assert.Equal(t, 3, ui.table.GetRowCount())

	ui.deviceItemSelected(1, 0)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "bbb")
}

func TestAnalyzePath(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc")
}

func TestAnalyzePathBW(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc")
}

func TestAnalyzePathWithParentDir(t *testing.T) {
	parentDir := &analyze.Dir{
		File: &analyze.File{
			Name: "parent",
		},
		Files: make([]fs.Item, 0, 1),
	}

	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, true, true, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.topDir = parentDir
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", parentDir)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())
	assert.Equal(t, parentDir, ui.currentDir.GetParent())

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ccc")
}

func TestReadAnalysis(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	input, err := os.OpenFile("../internal/testdata/test.json", os.O_RDONLY, 0644)
	assert.Nil(t, err)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, true, false, false)
	ui.done = make(chan struct{})

	err = ui.ReadAnalysis(input)
	assert.Nil(t, err)

	<-ui.done // wait for reading

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "gdu", ui.currentDir.GetName())
}

func TestReadAnalysisWithWrongFile(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	input, err := os.OpenFile("../internal/testdata/wrong.json", os.O_RDONLY, 0644)
	assert.Nil(t, err)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.done = make(chan struct{})

	err = ui.ReadAnalysis(input)
	assert.Nil(t, err)

	<-ui.done // wait for reading

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.True(t, ui.pages.HasPage("error"))
}

func TestViewDirContents(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	res := ui.showFile() // selected item is dir, do nothing
	assert.Nil(t, res)
}

func TestViewFileWithoutCurrentDir(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})

	res := ui.showFile() // no current directory
	assert.Nil(t, res)
}

func TestViewContentsOfNotExistingFile(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.table.Select(3, 0)

	selectedFile := ui.table.GetCell(3, 0).GetReference().(fs.Item)
	assert.Equal(t, "ddd", selectedFile.GetName())

	res := ui.showFile()
	assert.Nil(t, res)
}

func TestViewFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(2, 0)

	file := ui.showFile()
	assert.True(t, ui.pages.HasPage("file"))

	event := file.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'j', 0))
	assert.Equal(t, 'j', event.Rune())
}

func TestChangeCwd(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()
	cwd := ""

	opt := func(ui *UI) {
		ui.SetChangeCwdFn(func(p string) error {
			cwd = p
			return nil
		})
	}
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false, opt)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(1, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	assert.Equal(t, cwd, "test_dir/nested/subnested")
}

func TestChangeCwdWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()
	cwd := ""

	opt := func(ui *UI) {
		ui.SetChangeCwdFn(func(p string) error {
			cwd = p
			return errors.New("failed")
		})
	}
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false, opt)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(1, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	assert.Equal(t, cwd, "test_dir/nested/subnested")
}

func TestShowInfo(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))

	assert.True(t, ui.pages.HasPage("info"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0))

	assert.False(t, ui.pages.HasPage("info"))
}

func TestShowInfoBW(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))

	assert.True(t, ui.pages.HasPage("info"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))

	assert.False(t, ui.pages.HasPage("info"))
}

func TestShowInfoWithHardlinks(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	nested := ui.currentDir.GetFiles()[0].(*analyze.Dir)
	subnested := nested.Files[1].(*analyze.Dir)
	file := subnested.Files[0].(*analyze.File)
	file2 := nested.Files[0].(*analyze.File)
	file.Mli = 1
	file2.Mli = 1

	ui.currentDir.UpdateStats(ui.linkedItems)

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(1, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(1, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))

	assert.True(t, ui.pages.HasPage("info"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0))

	assert.False(t, ui.pages.HasPage("info"))
}

func TestShowInfoWithoutCurrentDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})

	// pressing `i` will do nothing
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'i', 0))
	assert.False(t, ui.pages.HasPage("info"))
}

func TestExitViewFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.table.Select(2, 0)

	file := ui.showFile()

	assert.True(t, ui.pages.HasPage("file"))

	file.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'q', 0))

	assert.False(t, ui.pages.HasPage("file"))
}
