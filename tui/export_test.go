package tui

import (
	"bytes"
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestConfirmExport(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.Analyzer = &testanalyze.MockedAnalyzer{}

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'E', 0))

	assert.True(t, ui.pages.HasPage("export"))

	ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'n', 0))
	ui.keyPressed(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	assert.True(t, ui.pages.HasPage("export"))
}

func TestExportAnalysis(t *testing.T) {
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

	ui.exportAnalysis()

	assert.True(t, ui.pages.HasPage("exporting"))

	<-ui.done

	assert.FileExists(t, "export.json")
	err := os.Remove("export.json")
	assert.NoError(t, err)

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}
}

func TestExportAnalysisEsc(t *testing.T) {
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

	form := ui.confirmExport()
	formInputFn := form.GetInputCapture()

	assert.True(t, ui.pages.HasPage("export"))

	formInputFn(tcell.NewEventKey(tcell.KeyEsc, 0, 0))

	assert.False(t, ui.pages.HasPage("export"))
}

func TestExportAnalysisWithName(t *testing.T) {
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

	form := ui.confirmExport()
	// formInputFn := form.GetInputCapture()
	item := form.GetFormItemByLabel("File name")
	inputFn := item.(*tview.InputField).InputHandler()

	// send 'n' to input
	inputFn(tcell.NewEventKey(tcell.KeyRune, 'n', 0), nil)
	assert.Equal(t, "export.jsonn", ui.exportName)

	assert.True(t, ui.pages.HasPage("export"))

	form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), nil)

	assert.True(t, ui.pages.HasPage("exporting"))

	<-ui.done

	assert.FileExists(t, "export.jsonn")
	err := os.Remove("export.jsonn")
	assert.NoError(t, err)

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}
}

func TestExportAnalysisWithoutRights(t *testing.T) {
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

	_, err := os.Create("export.json")
	assert.NoError(t, err)
	err = os.Chmod("export.json", 0)
	assert.NoError(t, err)
	defer func() {
		err = os.Chmod("export.json", 0o755)
		assert.Nil(t, err)
		err = os.Remove("export.json")
		assert.NoError(t, err)
	}()

	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false, false)
	ui.done = make(chan struct{})
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	ui.currentDir = currentDir
	ui.topDir = parentDir

	ui.exportAnalysis()

	<-ui.done

	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.True(t, ui.pages.HasPage("error"))
}
