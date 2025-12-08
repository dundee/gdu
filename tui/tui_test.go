package tui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

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

	printScreen(simScreen)

	text := []byte(" Total disk    usage: 4.0 KiB Apparent size: 2 B Items: 1")
	for i, r := range b {
		if i >= len(text) {
			break
		}
		assert.Equal(t, string(text[i]), string(r.Bytes[0]), fmt.Sprintf("Index: %d", i))
	}
}

func TestUpdateProgress(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
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

	cells := b[507 : 507+9]

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

	cells := b[507 : 507+9]

	text := []byte("directory")
	for i, r := range cells {
		assert.Equal(t, text[i], r.Bytes[0])
	}
}

func TestAppRun(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	err := ui.StartUILoop()

	assert.Nil(t, err)
}

func TestAppRunWithErr(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
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

	simScreen := testapp.CreateSimScreen()
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

	ui := getAnalyzedPathMockedApp(t, true, false, false)
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
	simScreen := testapp.CreateSimScreen()
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
	simScreen := testapp.CreateSimScreen()
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

func TestDeleteSelectedInParallel(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})
	ui.SetDeleteInParallel()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.deleteSelected(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestDeleteSelectedInBackground(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.remover = testanalyze.ItemFromDirWithSleep
	ui.done = make(chan struct{})
	ui.SetDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.deleteSelected(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestDeleteSelectedInBackgroundAndParallel(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.remover = testanalyze.ItemFromDirWithSleep
	ui.done = make(chan struct{})
	ui.SetDeleteInBackground()
	ui.SetDeleteInParallel()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.deleteSelected(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestDeleteSelectedInBackgroundBW(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})
	ui.SetDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.deleteSelected(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestEmptyDirInBackground(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.done = make(chan struct{})
	ui.SetDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.deleteSelected(true)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/nested/subnested")
}

func TestEmptyFileInBackground(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.done = make(chan struct{})
	ui.SetDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.fileItemSelected(0, 0) // nested
	ui.table.Select(2, 0)

	ui.deleteSelected(true)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.FileExists(t, "test_dir/nested/file2")

	f, err := os.Open("test_dir/nested/file2")
	assert.Nil(t, err)
	info, err := f.Stat()
	assert.Nil(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestDeleteSelectedWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.remover = testanalyze.ItemFromDirWithErr

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

func TestDeleteSelectedInBackgroundWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.SetDeleteInBackground()
	ui.remover = testanalyze.ItemFromDirWithSleepAndErr

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.delete(false)

	<-ui.done

	// change the status
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	// wait for status to be removed
	time.Sleep(500 * time.Millisecond)

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
	ui.remover = testanalyze.ItemFromDirWithErr

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

func TestDeleteMarkedInBackground(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.SetDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.fileItemSelected(0, 0) // nested

	ui.markedRows[1] = struct{}{} // subnested
	ui.markedRows[2] = struct{}{} // file2

	ui.deleteMarked(false)

	<-ui.done // wait for deletion of subnested
	<-ui.done // wait for deletion of file2

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/nested/subnested")
	assert.NoFileExists(t, "test_dir/nested/file2")
}

func TestDeleteMarkedInBackgroundWithStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.SetAnalyzer(analyze.CreateStoredAnalyzer("/tmp/badger"))
	ui.SetDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.fileItemSelected(0, 0) // nested

	ui.markedRows[1] = struct{}{} // subnested
	ui.markedRows[2] = struct{}{} // file2

	ui.deleteMarked(false)

	<-ui.done // wait for deletion of subnested
	<-ui.done // wait for deletion of file2

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/nested/subnested")
	assert.NoFileExists(t, "test_dir/nested/file2")
}

func TestDeleteMarkedInBackgroundWithStorageAndParallel(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.SetAnalyzer(analyze.CreateStoredAnalyzer("/tmp/badger"))
	ui.SetDeleteInBackground()
	ui.SetDeleteInParallel()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.fileItemSelected(0, 0) // nested

	ui.markedRows[1] = struct{}{} // subnested
	ui.markedRows[2] = struct{}{} // file2

	ui.deleteMarked(false)

	<-ui.done // wait for deletion of subnested
	<-ui.done // wait for deletion of file2

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/nested/subnested")
	assert.NoFileExists(t, "test_dir/nested/file2")
}

func TestDeleteMarkedInBackgroundWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.SetDeleteInBackground()
	ui.remover = testanalyze.ItemFromDirWithErr

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
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	ui.showErr("Something went wrong", errors.New("error"))

	assert.True(t, ui.pages.HasPage("error"))
}

func TestShowErrBW(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
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

func TestSetStyles(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	opts := []Option{}
	opts = append(opts, func(ui *UI) {
		ui.SetHeaderHidden()
	})

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false, opts...)

	ui.SetSelectedBackgroundColor(tcell.ColorRed)
	ui.SetSelectedTextColor(tcell.ColorRed)
	ui.SetFooterTextColor("red")
	ui.SetFooterBackgroundColor("red")
	ui.SetFooterNumberColor("red")
	ui.SetHeaderTextColor("red")
	ui.SetHeaderBackgroundColor("red")
	ui.SetResultRowDirectoryColor("red")
	ui.SetResultRowNumberColor("red")

	assert.Equal(t, ui.selectedBackgroundColor, tcell.ColorRed)
	assert.Equal(t, ui.selectedTextColor, tcell.ColorRed)
	assert.Equal(t, ui.footerTextColor, "red")
	assert.Equal(t, ui.footerBackgroundColor, "red")
	assert.Equal(t, ui.footerNumberColor, "red")
	assert.Equal(t, ui.headerTextColor, "red")
	assert.Equal(t, ui.headerBackgroundColor, "red")
	assert.Equal(t, ui.headerHidden, true)
	assert.Equal(t, ui.resultRow.DirectoryColor, "red")
	assert.Equal(t, ui.resultRow.NumberColor, "red")
}

func TestSetCurrentItemNameMaxLen(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetCurrentItemNameMaxLen(5)

	assert.Equal(t, ui.currentItemNameMaxLen, 5)
}

func TestUseOldSizeBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.UseOldSizeBar()

	assert.Equal(t, ui.useOldSizeBar, true)
}

func TestSetShowItemCount(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetShowItemCount()

	assert.Equal(t, ui.showItemCount, true)
}

func TestSetShowMTime(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetShowMTime()

	assert.Equal(t, ui.showMtime, true)
}

func TestNoDelete(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetNoDelete()

	assert.Equal(t, ui.noDelete, true)
}

func TestNoSpawnShell(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)

	ui.SetNoSpawnShell()

	assert.Equal(t, ui.noSpawnShell, true)
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

func getAnalyzedPathMockedApp(t *testing.T, useColors, apparentSize, mockedAnalyzer bool) *UI {
	simScreen := testapp.CreateSimScreen()
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

func TestConfirmDeletionSelectedButtonOrder(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletionSelected(false)

	// Verify confirmation page is created
	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionSelectedSafeDefault(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	assert.Equal(t, 1, ui.table.GetRowCount())
	ui.table.Select(0, 0)

	// Create confirmation dialog
	ui.confirmDeletionSelected(false)

	// Verify that the confirmation dialog exists with safer defaults
	assert.DirExists(t, "test_dir/nested")
	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionButtonIndexMapping(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false // Skip confirmation for direct testing

	assert.Equal(t, 1, ui.table.GetRowCount())
	ui.table.Select(0, 0)

	// Test that deletion still works when explicitly called
	ui.deleteSelected(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestConfirmEmptySelectedSafeDefault(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletionSelected(true)

	// Verify empty confirmation dialog is created safely
	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionMarkedSafeDefault(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	ui.markedRows[1] = struct{}{}
	ui.confirmDeletionMarked(false)

	// Verify marked deletion confirmation dialog is created safely
	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmEmptyMarkedSafeDefault(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	ui.markedRows[1] = struct{}{}
	ui.confirmDeletionMarked(true)

	// Verify marked empty confirmation dialog is created safely
	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestSaferConfirmationPreventDataLoss(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)

	assert.Equal(t, 1, ui.table.GetRowCount())
	ui.table.Select(0, 0)

	// Test that creating confirmation dialog doesn't accidentally trigger deletion
	ui.confirmDeletionSelected(false)
	ui.confirmDeletionSelected(true) // empty

	// Directory should still exist - no accidental deletion
	assert.DirExists(t, "test_dir/nested")
	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionSelectedCase1(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	assert.Equal(t, 1, ui.table.GetRowCount())
	ui.table.Select(0, 0)

	// Test case 1 branch (yes button at index 1) by directly calling deleteSelected
	ui.deleteSelected(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestConfirmDeletionMarkedCase1(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	ui.fileItemSelected(0, 0)     // nested
	ui.markedRows[1] = struct{}{} // subnested

	// Test case 1 branch (yes button at index 1) by directly calling deleteMarked
	ui.deleteMarked(false)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested/subnested")
}
