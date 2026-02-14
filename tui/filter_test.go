package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestFiltering(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	// mark the item for deletion
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, ' ', 0))
	assert.Equal(t, 1, len(ui.markedRows))

	ui.showFilterInput()
	ui.filterValue = ""
	ui.showDir()

	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc") // nothing is filtered
	// marking should be dropped after sorting
	assert.Equal(t, 0, len(ui.markedRows))

	ui.filterValue = "aa"
	ui.showDir()

	assert.Contains(t, ui.table.GetCell(0, 0).Text, "aaa") // shows only cccc

	ui.hideFilterInput()
	ui.showDir()

	assert.Contains(t, ui.table.GetCell(0, 0).Text, "ccc") // filtering reset
}

func TestFilteringWithoutCurrentDir(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})

	ui.showFilterInput()

	assert.False(t, ui.filtering)
}

func TestSwitchToTable(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
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
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done // wait for analyzer

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
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

func createDirWithExtensions() *analyze.Dir {
	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "test_dir",
			Usage: 1e9,
			Size:  1e9,
			Mtime: time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
		},
		BasePath:  ".",
		ItemCount: 6,
	}
	subdir := &analyze.Dir{
		File: &analyze.File{
			Name:   "subdir",
			Usage:  1e6,
			Size:   1e6,
			Mtime:  time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
			Parent: dir,
		},
	}
	goFile := &analyze.File{
		Name:   "main.go",
		Usage:  1e6,
		Size:   1e6,
		Mtime:  time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
		Parent: dir,
	}
	yamlFile := &analyze.File{
		Name:   "config.yaml",
		Usage:  1e3,
		Size:   1e3,
		Mtime:  time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
		Parent: dir,
	}
	jsonFile := &analyze.File{
		Name:   "data.json",
		Usage:  1e4,
		Size:   1e4,
		Mtime:  time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
		Parent: dir,
	}
	noExtFile := &analyze.File{
		Name:   "Makefile",
		Usage:  500,
		Size:   500,
		Mtime:  time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
		Parent: dir,
	}
	dir.Files = fs.Files{subdir, goFile, yamlFile, jsonFile, noExtFile}
	return dir
}

func TestTypeFiltering(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	dir := createDirWithExtensions()
	ui.currentDir = dir
	ui.topDir = dir
	ui.topDirPath = dir.GetPath()
	ui.showDir()

	rowCount := ui.table.GetRowCount()
	assert.Equal(t, 5, rowCount) // subdir + main.go + config.yaml + data.json + Makefile

	// activate type filter for "go" files
	ui.showTypeFilterInput()
	assert.True(t, ui.typeFiltering)

	ui.typeFilterValue = "go"
	ui.showDir()

	// should show: subdir (dirs always shown) + main.go
	assert.True(t, tableContains(ui, "subdir"))
	assert.True(t, tableContains(ui, "main.go"))
	assert.False(t, tableContains(ui, "config.yaml"))
	assert.False(t, tableContains(ui, "data.json"))
	assert.False(t, tableContains(ui, "Makefile"))

	ui.typeFilterValue = "go,yaml"
	ui.showDir()

	assert.True(t, tableContains(ui, "subdir"))
	assert.True(t, tableContains(ui, "main.go"))
	assert.True(t, tableContains(ui, "config.yaml"))
	assert.False(t, tableContains(ui, "data.json"))

	// hide type filter resets it
	ui.hideTypeFilterInput()
	ui.showDir()

	assert.True(t, tableContains(ui, "main.go"))
	assert.True(t, tableContains(ui, "config.yaml"))
	assert.True(t, tableContains(ui, "data.json"))
	assert.True(t, tableContains(ui, "Makefile"))
	assert.Empty(t, ui.typeFilterValue)
}

func TestTypeFilteringWithoutCurrentDir(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	ui.showTypeFilterInput()

	assert.False(t, ui.typeFiltering)
}

func TestTypeFilteringKeyBinding(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'T', 0))

	assert.True(t, ui.typeFiltering)
	assert.NotNil(t, ui.typeFilteringInput)
}

func TestExitTypeFiltering(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'T', 0))
	handler := ui.typeFilteringInput.InputHandler()
	ui.typeFilterValue = "go"
	ui.showDir()

	handler(
		tcell.NewEventKey(tcell.KeyEsc, ' ', 0), func(p tview.Primitive) {},
	)

	assert.Empty(t, ui.typeFilterValue)
	assert.Nil(t, ui.typeFilteringInput)
	assert.False(t, ui.typeFiltering)
}

func TestTypeFilterTabSwitch(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	// open type filter, confirm with Enter, then TAB should switch back
	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'T', 0))
	assert.True(t, ui.typeFiltering)

	handler := ui.typeFilteringInput.InputHandler()
	handler(
		tcell.NewEventKey(tcell.KeyEnter, ' ', 0), func(p tview.Primitive) {},
	)
	assert.False(t, ui.typeFiltering) // focus returned to table

	ui.keyPressed(tcell.NewEventKey(tcell.KeyTAB, ' ', 0))
	assert.True(t, ui.typeFiltering) // TAB should switch back to type filter
}

func TestBothFiltersCoexist(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.done = make(chan struct{})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	dir := createDirWithExtensions()
	ui.currentDir = dir
	ui.topDir = dir
	ui.topDirPath = dir.GetPath()

	// activate both filters
	ui.showFilterInput()
	ui.filterValue = "main"
	ui.showTypeFilterInput()
	ui.typeFilterValue = "go"
	ui.showDir()

	assert.True(t, tableContains(ui, "main.go"))    // matches both name "main" and type "go"
	assert.False(t, tableContains(ui, "subdir"))    // dir name doesn't contain "main"
	assert.False(t, tableContains(ui, "data.json")) // doesn't match name or type
}

func TestMatchesTypeFilter(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	ui.typeFilterValue = "go"
	assert.True(t, ui.matchesTypeFilter("main.go", false))
	assert.False(t, ui.matchesTypeFilter("config.yaml", false))
	assert.True(t, ui.matchesTypeFilter("subdir", true))     // dirs always match
	assert.False(t, ui.matchesTypeFilter("Makefile", false)) // no extension

	ui.typeFilterValue = "go,yaml"
	assert.True(t, ui.matchesTypeFilter("main.go", false))
	assert.True(t, ui.matchesTypeFilter("config.yaml", false))
	assert.False(t, ui.matchesTypeFilter("data.json", false))

	ui.typeFilterValue = ".go" // with leading dot
	assert.True(t, ui.matchesTypeFilter("main.go", false))

	ui.typeFilterValue = "GO" // case insensitive
	assert.True(t, ui.matchesTypeFilter("main.go", false))

	ui.typeFilterValue = "" // empty filter matches all
	assert.True(t, ui.matchesTypeFilter("anything", false))
}

func collectTableTexts(ui *UI) []string {
	var texts []string
	for i := 0; i < ui.table.GetRowCount(); i++ {
		cell := ui.table.GetCell(i, 0)
		if cell != nil {
			texts = append(texts, cell.Text)
		}
	}
	return texts
}

func tableContains(ui *UI, name string) bool {
	for _, text := range collectTableTexts(ui) {
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}
