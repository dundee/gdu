package tui

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestShowHelp(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '?', 0))

	assert.True(t, ui.pages.HasPage("help"))
}

func TestCloseHelp(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyEsc, 'q', 0))

	assert.False(t, ui.pages.HasPage("help"))
}

func TestCloseHelpWithQuestionMark(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '?', 0))

	assert.False(t, ui.pages.HasPage("help"))
}

func TestKeyWhileDeleting(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	modal := tview.NewModal().SetText("Deleting...")
	ui.pages.AddPage("deleting", modal, true, true)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyEnter, ' ', 0))
	assert.Equal(t, tcell.KeyEnter, key.Key())
}

func TestLeftRightKeyWhileConfirm(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	modal := tview.NewModal().SetText("Really?")
	ui.pages.AddPage("confirm", modal, true, true)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyLeft, 'h', 0))
	assert.Equal(t, tcell.KeyLeft, key.Key())
	key = ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	assert.Equal(t, tcell.KeyRight, key.Key())
}

func TestMoveLeftRight(t *testing.T) {
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
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	assert.Equal(t, "nested", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0)) // try /.. first

	assert.Equal(t, "nested", ui.currentDir.GetName())

	ui.table.Select(1, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	assert.Equal(t, "subnested", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyLeft, 'h', 0))

	assert.Equal(t, "nested", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyLeft, 'h', 0))

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyLeft, 'h', 0))

	assert.Equal(t, "test_dir", ui.currentDir.GetName())
}

func TestMoveRightOnDevice(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.SetIgnoreDirPaths([]string{})
	err := ui.ListDevices(getDevicesInfoMock())
	assert.Nil(t, err)

	ui.table.Select(1, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	// go back to list of devices
	ui.keyPressed(tcell.NewEventKey(tcell.KeyLeft, 'h', 0))

	assert.Nil(t, ui.currentDir)
	assert.Equal(t, "/dev/root", ui.table.GetCell(1, 0).GetReference().(*device.Device).Name)
}

func TestStop(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0))
	assert.Nil(t, key)
}

func TestStopWithPrintingPath(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	buff := &bytes.Buffer{}
	ui := CreateUI(app, simScreen, buff, true, true, false, false, false)

	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'Q', 0))
	assert.Nil(t, key)

	assert.Equal(t, "test_dir\n", buff.String())
}

func TestSpawnShell(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	buff := &bytes.Buffer{}
	ui := CreateUI(app, simScreen, buff, true, true, false, false, false)
	var called = false
	ui.exec = func(argv0 string, argv, envv []string) error {
		called = true
		return nil
	}

	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'b', 0))
	assert.Nil(t, key)
	assert.True(t, called)
}

func TestSpawnShellWithoutDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	buff := &bytes.Buffer{}
	ui := CreateUI(app, simScreen, buff, true, true, false, false, false)
	var called = false
	ui.exec = func(argv0 string, argv, envv []string) error {
		called = true
		return nil
	}

	ui.done = make(chan struct{})

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'b', 0))
	assert.Nil(t, key)
	assert.False(t, called)
}

func TestSpawnShellWithWrongDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	buff := &bytes.Buffer{}
	ui := CreateUI(app, simScreen, buff, true, true, false, false, false)
	var called = false
	ui.exec = func(argv0 string, argv, envv []string) error {
		called = true
		return nil
	}

	ui.done = make(chan struct{})
	ui.currentDir = &analyze.Dir{}
	ui.currentDirPath = "/xxxxx"

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'b', 0))
	assert.Nil(t, key)
	assert.False(t, called)
	assert.True(t, ui.pages.HasPage("error"))
}

func TestSpawnShellWithError(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	buff := &bytes.Buffer{}
	ui := CreateUI(app, simScreen, buff, true, true, false, false, false)
	var called = false
	ui.exec = func(argv0 string, argv, envv []string) error {
		called = true
		return errors.New("wrong shell")
	}

	ui.done = make(chan struct{})
	ui.currentDir = &analyze.Dir{}
	ui.currentDirPath = "."

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'b', 0))
	assert.Nil(t, key)
	assert.True(t, called)
	assert.True(t, ui.pages.HasPage("error"))
}

func TestShowConfirm(t *testing.T) {
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

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.table.Select(1, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	assert.True(t, ui.pages.HasPage("confirm"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '?', 0))

	assert.False(t, ui.pages.HasPage("help"))
}

func TestDeleteEmpty(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))
	assert.NotNil(t, key)
}

func TestMarkEmpty(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, ' ', 0))
	assert.NotNil(t, key)
}

func TestDelete(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestDeleteMarked(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, ' ', 0))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestDeleteParent(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())
	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	assert.DirExists(t, "test_dir/nested")
}

func TestMarkParent(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())
	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, ' ', 0))

	assert.Equal(t, len(ui.markedRows), 0)
}

func TestEmptyDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'e', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/nested/subnested")
}

func TestMarkedEmptyDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, ' ', 0))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'e', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/nested/subnested")
}

func TestEmptyFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0)) // into nested

	ui.table.Select(2, 0) // file2

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'e', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.DirExists(t, "test_dir/nested/subnested")
}

func TestMarkedEmptyFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0)) // into nested

	ui.table.Select(2, 0) // file2

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, ' ', 0))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'e', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.DirExists(t, "test_dir/nested/subnested")
}

func TestSortByApparentSize(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'a', 0))

	assert.True(t, ui.ShowApparentSize)
}

func TestShowFileCount(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'c', 0))

	assert.True(t, ui.showItemCount)
}

func TestShowFileCountBW(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'c', 0))

	assert.True(t, ui.showItemCount)
}

func TestShowMtime(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'm', 0))

	assert.True(t, ui.showMtime)
}

func TestShowMtimeBW(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'm', 0))

	assert.True(t, ui.showMtime)
}

func TestShowRelativeBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.Equal(t, "test_dir", ui.currentDir.GetName())
	assert.False(t, ui.ShowRelativeSize)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'B', 0))

	assert.True(t, ui.ShowRelativeSize)
}

func TestRescan(t *testing.T) {
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

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'r', 0))

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

func TestSorting(t *testing.T) {
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

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 's', 0))
	assert.Equal(t, "size", ui.sortBy)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'C', 0))
	assert.Equal(t, "itemCount", ui.sortBy)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'n', 0))
	assert.Equal(t, "name", ui.sortBy)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'M', 0))
	assert.Equal(t, "mtime", ui.sortBy)
}

func TestShowFile(t *testing.T) {
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
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'v', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0))
}

func TestShowInfoAndMoveAround(t *testing.T) {
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

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'k', 0)) // move up
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'j', 0)) // move down
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'k', 0)) // move up
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '?', 0)) // does nothing

	assert.True(t, ui.pages.HasPage("info")) // we can still see info page

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0))

	assert.False(t, ui.pages.HasPage("info"))
}
