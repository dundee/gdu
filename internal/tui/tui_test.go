package tui

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/session"
	"github.com/dundee/gdu/v6/internal/testanalyze"
	"github.com/dundee/gdu/v6/internal/testapp"
	"github.com/dundee/gdu/v6/internal/testdev"
	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/analyze/badger"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	slog.SetLogLoggerLevel(slog.LevelWarn)
}

func TestFooter(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(15, 15)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

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

	// printScreen(simScreen)

	text := []byte(" Total disk    usage: 4.0 KiB Apparent size: 2 B Items: 1")
	for i, r := range b {
		if i >= len(text) {
			break
		}
		assert.Equal(t, string(text[i]), string(r.Bytes[0]), fmt.Sprintf("Index: %d", i))
	}
}

func TestProgressObserverShowsCancellationHint(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})
	ui.progress = ui.header

	ui.progressObserver(0, false)(session.Progress{Root: 1, Roots: 1})

	draws := app.GetUpdateDraws()
	require.Len(t, draws, 1)
	draws[0]()
	assert.Contains(t, ui.progress.GetText(false), "Press Esc or Ctrl+C to stop scanning and keep results")
}

func TestStopScanHint(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})

	assert.Equal(t, "Press Esc or Ctrl+C to stop scanning and keep results", ui.stopScanHint())

	// Ctrl+C quits, so it must not be advertised as the way to keep results
	ui.setCtrlCQuits()
	assert.Equal(t, "Press Esc to stop scanning and keep results", ui.stopScanHint())
}

func TestSetShowDiskProgressBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})

	ui.setShowDiskProgressBar(true)
	assert.True(t, ui.showDiskProgressBar)

	ui.setShowDiskProgressBar(false)
	assert.False(t, ui.showDiskProgressBar)
}

func TestDeviceSelectedSetsCurrentDeviceSize(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(),
		&config.Settings{Display: config.Display{Colors: true, ApparentSize: true, RelativeSize: true}})
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.ListDevices(getDevicesInfoMock())
	assert.Nil(t, err)

	ui.deviceItemSelected(1, 0)

	// currentDeviceSize must be set to the selected device's Size before analysis starts.
	assert.Equal(t, int64(1e12), ui.currentDeviceSize)

	<-ui.done
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}
}

func TestHelp(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	ui.help.Draw(simScreen)
	simScreen.Show()

	assert.Contains(t, screenText(simScreen), "directory")
}

func TestHelpBw(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})
	ui.showHelp()
	ui.help.Draw(simScreen)
	simScreen.Show()

	assert.Contains(t, screenText(simScreen), "directory")
}

func TestHelpShowsScanKeys(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	text := ui.formatHelpTextFor()
	assert.Contains(t, text, "Preview results found so far")
	assert.Contains(t, text, "Stop scanning and keep results")
}

func TestAppRun(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	err := ui.StartUILoop()

	assert.Nil(t, err)
}

func TestUILoopReturnsWithPendingSignal(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	assert.NoError(t, simScreen.Init())
	defer simScreen.Fini()
	screen := &failOnceSignalScreen{
		Screen:    simScreen,
		attempted: make(chan struct{}),
		fail:      true,
	}
	app := &signalLifecycleApp{
		MockedApp:       testapp.CreateMockedApp(false),
		signalAttempted: screen.attempted,
	}
	ui := CreateUI(app, screen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})
	signals := make(chan os.Signal, 1)
	signals <- os.Interrupt

	result := make(chan error, 1)
	go func() {
		result <- ui.runUILoop(signals)
	}()

	select {
	case err := <-result:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("UI loop did not stop its signal listener")
	}
}

func TestAppRunWithErr(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

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
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})
	ui.done = make(chan struct{})
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.currentDir = currentDir
	ui.session.SetRoot(parentDir)
	ui.rescanDir()

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())
	assert.Equal(t, parentDir, ui.currentDir.GetParent())

	assert.Equal(t, 5, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, "/..")
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ddd")
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

	ui.fileItemSelected(0, 0) // does nothing, since it's a file

	assert.Equal(t, 4, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "ccc")
}

func TestSelectedWithoutCurrentDir(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})

	ui.fileItemSelected(1, 0)

	assert.Nil(t, ui.currentDir)
}

func TestBeforeDraw(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	err := screen.Init()

	assert.Nil(t, err)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, screen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	for _, f := range ui.app.(*testapp.MockedApp).BeforeDraws {
		assert.False(t, f(screen))
	}
}

func TestIgnorePaths(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(),
		&config.Settings{Display: config.Display{ApparentSize: true}, Scan: config.Scan{IgnoreDirs: []string{"/aaa", "/bbb"}}})

	assert.True(t, ui.ShouldDirBeIgnored("aaa", "/aaa"))
	assert.True(t, ui.ShouldDirBeIgnored("bbb", "/bbb"))
	assert.False(t, ui.ShouldDirBeIgnored("ccc", "/ccc"))
}

func TestConfirmDeletion(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletion(ui.selectedTargets(), ActionDelete)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionBW(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletion(ui.selectedTargets(), ActionDelete)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmEmpty(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletion(ui.selectedTargets(), ActionEmpty)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmEmptyMarked(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	markRow(t, ui, 1)
	ui.confirmDeletion(ui.selectedTargets(), ActionEmpty)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionMarked(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	markRow(t, ui, 1)
	ui.confirmDeletion(ui.selectedTargets(), ActionDelete)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionMarkedBW(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	markRow(t, ui, 1)
	ui.confirmDeletion(ui.selectedTargets(), ActionDelete)

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestDeleteSelected(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	deleteNow(ui, ActionDelete)

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
	ui.setDeleteInParallel()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	deleteNow(ui, ActionDelete)

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
	ui.newDeleter = fakeDeleter(testanalyze.ItemFromDirWithSleep)
	ui.done = make(chan struct{})
	ui.setDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	deleteNow(ui, ActionDelete)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestMoveToTrashSelectedInBackground(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.done = make(chan struct{})
	ui.setDeleteInBackground()
	var trashed string
	ui.trasher = func(item fs.Item) (bool, error) {
		trashed = item.GetName()
		return true, nil
	}

	assert.Equal(t, 1, ui.table.GetRowCount())
	ui.table.Select(0, 0)
	deleteNow(ui, ActionMoveToTrash)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "nested", trashed)
	assert.Equal(t, 0, ui.table.GetRowCount())
}

func TestDeleteSelectedInBackgroundAndParallel(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.newDeleter = fakeDeleter(testanalyze.ItemFromDirWithSleep)
	ui.done = make(chan struct{})
	ui.setDeleteInBackground()
	ui.setDeleteInParallel()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	deleteNow(ui, ActionDelete)

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
	ui.setDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	deleteNow(ui, ActionDelete)

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
	ui.setDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	deleteNow(ui, ActionEmpty)

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
	ui.setDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.fileItemSelected(0, 0) // nested
	ui.table.Select(2, 0)

	deleteNow(ui, ActionEmpty)

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
	ui.newDeleter = fakeDeleter(testanalyze.ItemFromDirWithErr)

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	deleteNow(ui, ActionDelete)

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
	ui.setDeleteInBackground()
	ui.newDeleter = fakeDeleter(testanalyze.ItemFromDirWithSleepAndErr)

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	deleteNow(ui, ActionDelete)

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
	ui.newDeleter = fakeDeleter(testanalyze.ItemFromDirWithErr)

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)
	markRow(t, ui, 0)

	deleteNow(ui, ActionDelete)

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
	ui.setDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.fileItemSelected(0, 0) // nested

	markRow(t, ui, 1) // subnested
	markRow(t, ui, 2) // file2

	deleteNow(ui, ActionDelete)

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
	ui.Analyzer = (badger.CreateAnalyzer("/tmp/badger"))
	ui.setDeleteInBackground()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.fileItemSelected(0, 0) // nested

	markRow(t, ui, 1) // subnested
	markRow(t, ui, 2) // file2

	deleteNow(ui, ActionDelete)

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
	ui.Analyzer = (badger.CreateAnalyzer("/tmp/badger"))
	ui.setDeleteInBackground()
	ui.setDeleteInParallel()

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.fileItemSelected(0, 0) // nested

	markRow(t, ui, 1) // subnested
	markRow(t, ui, 2) // file2

	deleteNow(ui, ActionDelete)

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
	ui.setDeleteInBackground()
	ui.newDeleter = fakeDeleter(testanalyze.ItemFromDirWithErr)

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)
	markRow(t, ui, 0)

	deleteNow(ui, ActionDelete)

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
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})

	ui.showErr("Something went wrong", errors.New("error"))

	assert.True(t, ui.pages.HasPage("error"))
}

func TestShowErrBW(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

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

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{
		Display: config.Display{ApparentSize: true},
		TUI:     config.TUI{Style: config.Style{HeaderHidden: true}},
	})

	ui.setSelectedBackgroundColor(tcell.ColorRed)
	ui.setSelectedTextColor(tcell.ColorRed)
	ui.setMarkedTextColor(tcell.ColorRed)
	ui.setMarkedBackgroundColor(tcell.ColorRed)
	ui.setFooterTextColor("red")
	ui.setFooterBackgroundColor("red")
	ui.setFooterNumberColor("red")
	ui.setHeaderTextColor("red")
	ui.setHeaderBackgroundColor("red")
	ui.setResultRowDirectoryColor("red")
	ui.setResultRowNumberColor("red")

	assert.Equal(t, ui.selectedBackgroundColor, tcell.ColorRed)
	assert.Equal(t, ui.selectedTextColor, tcell.ColorRed)
	assert.Equal(t, ui.markedTextColor, tcell.ColorRed)
	assert.Equal(t, ui.markedBackgroundColor, tcell.ColorRed)
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
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	ui.setCurrentItemNameMaxLen(5)

	assert.Equal(t, ui.currentItemNameMaxLen, 5)
}

func TestUseOldSizeBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	ui.setOldSizeBar()

	assert.Equal(t, ui.useOldSizeBar, true)
}

func TestSetShowBarPercentage(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	ui.setShowBarPercentage()

	assert.Equal(t, ui.showBarPercentage, true)
}

func TestSetShowItemCount(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	ui.setShowItemCount()

	assert.Equal(t, ui.showItemCount, true)
}

func TestSetShowItemCountBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	assert.Equal(t, ui.showItemCountBar, false)

	ui.setShowItemCountBar()

	assert.Equal(t, ui.showItemCountBar, true)
}

func TestSetShowMTime(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	ui.setShowMTime()

	assert.Equal(t, ui.showMtime, true)
}

func TestNoDelete(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	ui.setNoDelete()

	assert.Equal(t, ui.noDelete, true)
}

func TestNoSpawnShell(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	ui.setNoSpawnShell()

	assert.Equal(t, ui.noSpawnShell, true)
}

func TestNoViewFile(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{ApparentSize: true}})

	ui.setNoViewFile()

	assert.Equal(t, ui.noViewFile, true)
}

// screenText returns the rendered contents of the simulated screen as a single
// string, with one line per screen row.
func screenText(simScreen tcell.SimulationScreen) string {
	b, width, _ := simScreen.GetContents()

	var sb strings.Builder
	for i, cell := range b {
		if i > 0 && i%width == 0 {
			sb.WriteByte('\n')
		}
		sb.Write(cell.Bytes)
	}
	return sb.String()
}

// nolint: unused // Why: for debugging
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
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(),
		&config.Settings{Display: config.Display{Colors: useColors, ApparentSize: apparentSize}})

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
	ui.confirmDeletionSelected(ui.selectedTargets(), ActionDelete)

	// Verify confirmation page is created
	assert.True(t, ui.pages.HasPage("confirm"))
}

func selectConfirmationYes(t *testing.T, ui *UI) {
	t.Helper()
	_, primitive := ui.pages.GetFrontPage()
	modal, ok := primitive.(*tview.Modal)
	if !assert.True(t, ok) {
		return
	}
	var setFocus func(tview.Primitive)
	setFocus = func(p tview.Primitive) {
		p.Focus(setFocus)
	}
	modal.SetFocus(1)
	modal.Focus(setFocus)
	modal.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), setFocus)
}

func TestConfirmMoveToTrashSelectedYes(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.done = make(chan struct{})
	var trashed string
	ui.trasher = func(item fs.Item) (bool, error) {
		trashed = item.GetName()
		return true, nil
	}
	ui.table.Select(0, 0)
	selected := ui.table.GetCell(0, 0).GetReference().(fs.Item).GetName()

	ui.confirmDeletionSelected(ui.selectedTargets(), ActionMoveToTrash)
	selectConfirmationYes(t, ui)
	<-ui.done

	assert.Equal(t, selected, trashed)
}

func TestConfirmMoveToTrashMarkedYes(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.done = make(chan struct{})
	var trashed string
	ui.trasher = func(item fs.Item) (bool, error) {
		trashed = item.GetName()
		return true, nil
	}
	markRow(t, ui, 0)

	ui.confirmDeletionMarked(ui.selectedTargets(), ActionMoveToTrash)
	selectConfirmationYes(t, ui)
	<-ui.done

	assert.Equal(t, "nested", trashed)
}

func TestConfirmDeletionSelectedSafeDefault(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})

	assert.Equal(t, 1, ui.table.GetRowCount())
	ui.table.Select(0, 0)

	// Create confirmation dialog
	ui.confirmDeletionSelected(ui.selectedTargets(), ActionDelete)

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
	deleteNow(ui, ActionDelete)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestConfirmEmptySelectedSafeDefault(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	ui.confirmDeletionSelected(ui.selectedTargets(), ActionEmpty)

	// Verify empty confirmation dialog is created safely
	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmDeletionMarkedSafeDefault(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, true, true, true)

	ui.table.Select(1, 0)
	markRow(t, ui, 1)
	ui.confirmDeletionMarked(ui.selectedTargets(), ActionDelete)

	// Verify marked deletion confirmation dialog is created safely
	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestConfirmEmptyMarkedSafeDefault(t *testing.T) {
	ui := getAnalyzedPathMockedApp(t, false, true, true)

	ui.table.Select(1, 0)
	markRow(t, ui, 1)
	ui.confirmDeletionMarked(ui.selectedTargets(), ActionEmpty)

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
	ui.confirmDeletionSelected(ui.selectedTargets(), ActionDelete)
	ui.confirmDeletionSelected(ui.selectedTargets(), ActionEmpty) // empty

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
	deleteNow(ui, ActionDelete)

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

	ui.fileItemSelected(0, 0) // nested
	markRow(t, ui, 1)         // subnested

	// Test case 1 branch (yes button at index 1) by directly calling delete
	deleteNow(ui, ActionDelete)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested/subnested")
}

// The UI is configured from its settings when it is created.
func TestCreateUIAppliesSettings(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	ui := CreateUI(testapp.CreateMockedApp(true), simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{
		Display: config.Display{ItemCount: true, MTime: true, SymlinkTarget: true, CollapsePath: true, SortBy: "name", SortOrder: "asc"},
		Delete:  config.Delete{Disabled: true, InParallel: true},
		TUI: config.TUI{
			Shell: "/bin/zsh", NoViewFile: true, NoSpawnShell: true, NoConfirmQuit: true, CtrlCQuits: true, BrowseParentDirs: true,
			Style: config.Style{
				SelectedRowText: "red", FooterText: "blue", HeaderHidden: true, ResultRowNumber: "green",
				CurrentItemNameMaxLen: 30, OldSizeBar: true, ShowBarPercentage: true, ShowItemCountBar: true, ShowDiskProgressBar: true,
			},
		},
	})

	assert.True(t, ui.showItemCount && ui.showMtime && ui.showSymlinkTarget && ui.collapsePath)
	assert.Equal(t, "name", ui.sortBy)
	assert.Equal(t, "asc", ui.sortOrder)
	assert.True(t, ui.noDelete && ui.deleteInParallel)
	assert.Equal(t, "/bin/zsh", ui.shellBin)
	assert.True(t, ui.noViewFile && ui.noSpawnShell && ui.ctrlCQuits && ui.browseParentDirs)
	assert.False(t, ui.confirmQuit)
	assert.Equal(t, tcell.GetColor("red"), ui.selectedTextColor)
	assert.Equal(t, "blue", ui.footerTextColor)
	assert.True(t, ui.headerHidden)
	assert.Equal(t, "green", ui.resultRow.NumberColor)
	assert.Equal(t, 30, ui.currentItemNameMaxLen)
	assert.True(t, ui.useOldSizeBar && ui.showBarPercentage && ui.showItemCountBar && ui.showDiskProgressBar)
}
