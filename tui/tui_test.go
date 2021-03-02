package tui

import (
	"errors"
	"runtime"
	"testing"

	"github.com/dundee/gdu/v4/analyze"
	"github.com/dundee/gdu/v4/device"
	"github.com/dundee/gdu/v4/internal/testanalyze"
	"github.com/dundee/gdu/v4/internal/testapp"
	"github.com/dundee/gdu/v4/internal/testdev"
	"github.com/dundee/gdu/v4/internal/testdir"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestFooter(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(15, 15)
	defer simScreen.Fini()

	ui := CreateUI(app, false, true)

	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "xxx",
			Size:  5,
			Usage: 4096,
		},
		BasePath:  ".",
		ItemCount: 2,
	}

	file := &analyze.File{
		Name:   "yyy",
		Size:   2,
		Usage:  4096,
		Parent: dir,
	}
	dir.Files = analyze.Files{file}

	ui.currentDir = dir
	ui.showDir()
	ui.pages.HidePage("progress")

	ui.footer.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte(" Total disk usage: 4.0 KiB Apparent size: 5 B Items: 2")
	for i, r := range b {
		if i >= len(text) {
			break
		}
		assert.Equal(t, string(text[i]), string(r.Bytes[0]))
	}
}

func TestUpdateProgress(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(15, 15)
	defer simScreen.Fini()

	ui := CreateUI(app, false, false)
	progress := ui.analyzer.GetProgress()
	progress.Done = true
	progress.CurrentItemName = "xxx"
	ui.updateProgress()
	assert.True(t, true)
}

func TestHelp(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, true, true)
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	ui.help.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	cells := b[356 : 356+9]

	text := []byte("directory")
	for i, r := range cells {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestHelpBw(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, false, true)
	ui.showHelp()
	ui.help.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	cells := b[356 : 356+9]

	text := []byte("directory")
	for i, r := range cells {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

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

func TestAppRun(t *testing.T) {
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, false, true)

	err := ui.StartUILoop()

	assert.Nil(t, err)
}

func TestAppRunWithErr(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)

	err := ui.StartUILoop()

	assert.Equal(t, "Fail", err.Error())
}

func TestAnalyzePath(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
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

func TestRescanDir(t *testing.T) {
	parentDir := &analyze.Dir{
		File: &analyze.File{
			Name: "parent",
		},
		Files: make([]analyze.Item, 0, 1),
	}
	currentDir := &analyze.Dir{
		File: &analyze.File{
			Name:   "sub",
			Parent: parentDir,
		},
	}

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	ui.analyzer = &testanalyze.MockedAnalyzer{}
	ui.currentDir = currentDir
	ui.topDir = parentDir
	ui.rescanDir()

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

func TestDirSelected(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.done = make(chan struct{})

	ui.fileItemSelected(0, 0)

	assert.Equal(t, 3, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "subnested")
}

func TestFileSelected(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.fileItemSelected(4, 0)

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "aaa")
}

func TestBeforeDraw(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)

	for _, f := range ui.app.(*testapp.MockedApp).BeforeDraws {
		assert.False(t, f(screen))
	}
}

func TestIgnorePaths(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)

	ui.SetIgnoreDirPaths([]string{"/aaa", "/bbb"})

	assert.True(t, ui.ShouldDirBeIgnored("/aaa"))
	assert.True(t, ui.ShouldDirBeIgnored("/bbb"))
	assert.False(t, ui.ShouldDirBeIgnored("/ccc"))
}

func TestConfirmDeletion(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletion()

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionBW(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletion()

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestDeleteSelected(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.deleteSelected()

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestDeleteSelectedWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.remover = testanalyze.RemoveItemFromDirWithErr

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.deleteSelected()

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.True(t, ui.pages.HasPage("error"))
	assert.DirExists(t, "test_dir/nested")
}

func TestShowErr(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, true, true)

	ui.showErr("Something went wrong", errors.New("error"))

	assert.True(t, ui.pages.HasPage("error"))
}

func TestShowErrBW(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)

	ui.showErr("Something went wrong", errors.New("error"))

	assert.True(t, ui.pages.HasPage("error"))
}

func TestMin(t *testing.T) {
	assert.Equal(t, 2, min(2, 5))
	assert.Equal(t, 3, min(4, 3))
}

func printScreen(simScreen tcell.SimulationScreen) {
	b, _, _ := simScreen.GetContents()

	for i, r := range b {
		println(i, string(r.Bytes))
	}
}

func analyzeMock(path string, progress *analyze.CurrentProgress, ignore analyze.ShouldDirBeIgnored) *analyze.Dir {
	return &analyze.Dir{
		File: &analyze.File{
			Name: "xxx",
		},
		BasePath: ".",
	}
}

func getDevicesInfoMock() device.DevicesInfoGetter {
	item := &device.Device{
		Name:       "/dev/root",
		MountPoint: "/",
		Size:       1e9,
		Free:       1e3,
	}
	item2 := &device.Device{
		Name:       "/dev/boot",
		MountPoint: "/boot",
		Size:       1e12,
		Free:       1e6,
	}

	mock := testdev.DevicesInfoGetterMock{}
	mock.Devices = []*device.Device{item, item2}
	return mock
}

func getAnalyzedPathMockedApp(t *testing.T, useColors, apparentSize bool, mockedAnalyzer bool) *UI {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, useColors, apparentSize)

	if mockedAnalyzer {
		ui.analyzer = &testanalyze.MockedAnalyzer{}
	}
	ui.done = make(chan struct{})
	ui.AnalyzePath("test_dir", nil)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	return ui
}
