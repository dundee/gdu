package tui

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestFiltering(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.showFilterInput()
	ui.filterValue = ""
	ui.showDir()

	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa") // nothing is filtered

	ui.filterValue = "cc"
	ui.showDir()

	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc") // shows only cccc

	ui.hideFilterInput()
	ui.showDir()

	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa") // filtering reset
}

func TestFilteringWithoutCurrentDir(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})

	ui.showFilterInput()

	assert.False(t, ui.filtering)
}

func TestSwitchToTable(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '/', 0)) // open filtering input
	handler := ui.filteringInput.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyRune, 'n', 0), func(p tview.Primitive) {})
	handler(tcell.NewEventKey(tcell.KeyRune, 'e', 0), func(p tview.Primitive) {})
	handler(tcell.NewEventKey(tcell.KeyRune, 's', 0), func(p tview.Primitive) {})

	ui.table.Select(0, 0)
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0)) // we are filtering, should do nothing

	assert.Contains(t, ui.table.GetCell(0, 0).Text, "nested")

	handler(
		tcell.NewEventKey(tcell.KeyTAB, ' ', 0), func(p tview.Primitive) {},
	) // switch focus to table
	ui.keyPressed(tcell.NewEventKey(tcell.KeyTAB, ' ', 0)) // switch back to input
	handler(
		tcell.NewEventKey(tcell.KeyEnter, ' ', 0), func(p tview.Primitive) {},
	) // switch back to table

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRight, 'l', 0)) // open nested dir

	assert.Contains(t, ui.table.GetCell(1, 0).Text, "subnested")
	assert.Empty(t, ui.filterValue) // filtering reset
}

func TestExitFiltering(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).UpdateDraws {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, '/', 0)) // open filtering input
	handler := ui.filteringInput.InputHandler()
	ui.filterValue = "xxx"
	ui.showDir()

	assert.Equal(t, ui.table.GetCell(0, 0).Text, "") // nothing is filtered

	handler(
		tcell.NewEventKey(tcell.KeyEsc, ' ', 0), func(p tview.Primitive) {},
	) // exit filtering

	assert.Contains(t, ui.table.GetCell(0, 0).Text, "nested")
	assert.Empty(t, ui.filterValue) // filtering reset
}
