package tui

import (
	"bytes"
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestFooter(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(15, 15)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

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
	dir.Files = fs.Files{file}

	ui.currentDir = dir
	ui.showDir()
	ui.pages.HidePage("progress")

	ui.footerLabel.Draw(simScreen)
	simScreen.Show()

	b, _, _ := simScreen.GetContents()

	text := []byte(" Total disk usage: 4.0 KiB Apparent size: 2 B Items: 1")
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

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false, false)
	done := ui.Analyzer.GetDone()
	done.Broadcast()
	ui.updateProgress()
	assert.True(t, true)
}

func TestHelp(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	ui.help.Draw(simScreen)
	simScreen.Show()

	// printScreen(simScreen)

	b, _, _ := simScreen.GetContents()

	cells := b[406 : 406+9]

	text := []byte("directory")
	for i, r := range cells {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestHelpBw(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.showHelp()
	ui.help.Draw(simScreen)
	simScreen.Show()

	// printScreen(simScreen)

	b, _, _ := simScreen.GetContents()

	cells := b[406 : 406+9]

	text := []byte("directory")
	for i, r := range cells {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestAppRun(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	err := ui.StartUILoop()

	assert.Nil(t, err)
}

func TestAppRunWithErr(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	err := ui.StartUILoop()

	assert.Equal(t, "Fail", err.Error())
}

func TestRescanDir(t *testing.T) {
	parentDir := &analyze.Dir{
		File: &analyze.File{
			Name: "parent",
		},
		Files: make([]fs.Item, 0, 1),
	}
	currentDir := &analyze.Dir{
		File: &analyze.File{
			Name:   "sub",
			Parent: parentDir,
		},
	}

	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.currentDir = currentDir
	ui.topDir = parentDir
	ui.rescanDir()

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

	ui.fileItemSelected(3, 0)

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc")
}

func TestSelectedWithoutCurrentDir(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	ui.fileItemSelected(1, 0)

	assert.Nil(t, ui.currentDir)
}

func TestBeforeDraw(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	err := screen.Init()

	assert.Nil(t, err)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, screen, &bytes.Buffer{}, false, true, false, false, false)

	for _, f := range ui.app.(*testapp.MockedApp).BeforeDraws {
		assert.False(t, f(screen))
	}
}

func TestIgnorePaths(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetIgnoreDirPaths([]string{"/aaa", "/bbb"})

	assert.True(t, ui.ShouldDirBeIgnored("aaa", "/aaa"))
	assert.True(t, ui.ShouldDirBeIgnored("bbb", "/bbb"))
	assert.False(t, ui.ShouldDirBeIgnored("ccc", "/ccc"))
}

func TestConfirmDeletion(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletion(false)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionBW(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletion(false)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmEmpty(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletion(true)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmEmptyMarked(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	ui.markedRows[1] = struct{}{}
	ui.confirmDeletion(true)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionMarked(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	ui.markedRows[1] = struct{}{}
	ui.confirmDeletion(false)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionMarkedBW(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	ui.markedRows[1] = struct{}{}
	ui.confirmDeletion(false)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestDeleteSelected(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.deleteSelected(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
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

	ui.delete(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.True(t, ui.pages.HasPage("error"))
	assert.DirExists(t, "test_dir/nested")
}

func TestDeleteMarkedWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.remover = testanalyze.RemoveItemFromDirWithErr

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)
	ui.markedRows[0] = struct{}{}

	ui.deleteMarked(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.True(t, ui.pages.HasPage("error"))
	assert.DirExists(t, "test_dir/nested")
}

func TestShowErr(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	ui.showErr("Something went wrong", errors.New("error"))

	assert.True(t, ui.pages.HasPage("error"))
}

func TestShowErrBW(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.showErr("Something went wrong", errors.New("error"))

	assert.True(t, ui.pages.HasPage("error"))
}

func TestMin(t *testing.T) {
	assert.Equal(t, 2, min(2, 5))
	assert.Equal(t, 3, min(4, 3))
}

func TestSetSelectedBackgroundColor(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetSelectedBackgroundColor(tcell.ColorRed)

	assert.Equal(t, ui.selectedBackgroundColor, tcell.ColorRed)
}

func TestSetSelectedTextColor(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetSelectedTextColor(tcell.ColorRed)

	assert.Equal(t, ui.selectedTextColor, tcell.ColorRed)
}

func TestSetCurrentItemNameMaxLen(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetCurrentItemNameMaxLen(5)

	assert.Equal(t, ui.currentItemNameMaxLen, 5)
}

// nolint: deadcode,unused // Why: for debugging
func printScreen(simScreen tcell.SimulationScreen) {
	b, _, _ := simScreen.GetContents()

	for i, r := range b {
		if string(r.Bytes) != " " {
			println(i, string(r.Bytes))
		}
	}
}

func getDevicesInfoMock() device.DevicesInfoGetter {
	item := &device.Device{
		Name:       "/dev/root",
		MountPoint: "test_dir",
		Size:       1e12,
		Free:       1e6,
	}
	item2 := &device.Device{
		Name:       "/dev/boot",
		MountPoint: "/boot",
		Size:       1e6,
		Free:       1e3,
	}

	mock := testdev.DevicesInfoGetterMock{}
	mock.Devices = []*device.Device{item, item2}
	return mock
}

func getAnalyzedPathMockedApp(t *testing.T, useColors, apparentSize bool, mockedAnalyzer bool) *UI {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, useColors, apparentSize, false, false, false)

	if mockedAnalyzer {
		ui.Analyzer = &testanalyze.MockedAnalyzer{}
	}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	return ui
}
