package tui

import (
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestShowHelp(t *testing.T) {
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '?', 0))

	assert.True(t, ui.pages.HasPage("help"))
}

func TestCloseHelp(t *testing.T) {
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyEsc, 'q', 0))

	assert.False(t, ui.pages.HasPage("help"))
}

func TestKeyWhileDeleting(t *testing.T) {
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)

	modal := tview.NewModal().SetText("Deleting...")
	ui.pages.AddPage("deleting", modal, true, true)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyEnter, ' ', 0))
	assert.Equal(t, tcell.KeyEnter, key.Key())
}

func TestLeftRightKeyWhileConfirm(t *testing.T) {
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)

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

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	assert.Equal(t, "nested", ui.currentDir.Name)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0)) // try /.. first

	assert.Equal(t, "nested", ui.currentDir.Name)

	ui.table.Select(1, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	assert.Equal(t, "subnested", ui.currentDir.Name)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyLeft, 'h', 0))

	assert.Equal(t, "nested", ui.currentDir.Name)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyLeft, 'h', 0))

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyLeft, 'h', 0))

	assert.Equal(t, "test_dir", ui.currentDir.Name)
}

func TestMoveRightOnDevice(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	ui.SetIgnoreDirPaths([]string{})
	err := ui.ListDevices(getDevicesInfoMock())
	assert.Nil(t, err)

	ui.table.Select(1, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)
}

func TestStop(t *testing.T) {
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, true, true)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0))
	assert.Nil(t, key)
}

func TestShowConfirm(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, true, true)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.PathChecker = testdir.MockedPathChecker
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.table.Select(1, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	assert.True(t, ui.pages.HasPage("confirm"))
}

func TestDeleteEmpty(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))
	assert.Nil(t, key)
}

func TestDelete(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.NoDirExists(t, "test_dir/nested")
}

func TestDeleteParent(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'd', 0))

	assert.DirExists(t, "test_dir/nested")
}

func TestEmptyDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'e', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/nested/subnested")
}

func TestEmptyFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.done = make(chan struct{})
	ui.askBeforeDelete = false
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.Equal(t, 1, ui.table.GetRowCount())

	ui.table.Select(0, 0)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0)) // into nested

	ui.table.Select(2, 0) // file2

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'e', 0))

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	assert.DirExists(t, "test_dir/nested")
	assert.DirExists(t, "test_dir/nested/subnested")
}

func TestSortByApparentSize(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.PathChecker = testdir.MockedPathChecker
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'a', 0))

	assert.True(t, ui.ShowApparentSize)
}

func TestShowFileCount(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, true, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.PathChecker = testdir.MockedPathChecker
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'c', 0))

	assert.True(t, ui.showItemCount)
}

func TestShowFileCountBW(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.PathChecker = testdir.MockedPathChecker
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'c', 0))

	assert.True(t, ui.showItemCount)
}

func TestRescan(t *testing.T) {
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
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.currentDir = currentDir
	ui.topDir = parentDir

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'r', 0))

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

func TestSorting(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, true)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.PathChecker = testdir.MockedPathChecker
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	assert.Equal(t, "test_dir", ui.currentDir.Name)

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 's', 0))
	assert.Equal(t, "size", ui.sortBy)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'C', 0))
	assert.Equal(t, "itemCount", ui.sortBy)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'n', 0))
	assert.Equal(t, "name", ui.sortBy)
}

func TestShowFile(t *testing.T) {
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
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'v', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'q', 0))
}
