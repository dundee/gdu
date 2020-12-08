package main

import (
	"path"
	"github.com/rivo/tview"
)


type UI struct {
	app *tview.Application
	header *tview.TextView
	dirContent *tview.Table
	topDir string
	currentDir string
}

func (ui *UI) ItemSelected(row, column int) {
	selectedDir := ui.dirContent.GetCell(row, column).Text
	ui.ShowDir(selectedDir)
} 

func CreateUI(topDir string) *UI {
	ui := &UI{}
	ui.topDir = path.Clean(topDir)

	ui.app = tview.NewApplication()

	ui.header = tview.NewTextView()

	ui.dirContent = tview.NewTable().SetSelectable(true, true)
	ui.dirContent.SetSelectedFunc(ui.ItemSelected)

	footer := tview.NewTextView().SetText("footer")

	grid := tview.NewGrid().SetRows(1, 0, 1)
	grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.dirContent, 1, 0, 1, 1, 0, 0, true).
		AddItem(footer, 2, 0, 1, 1, 0, 0, false)
	
	modal := tview.NewModal().SetText("bbb")

	pages := tview.NewPages().
		AddPage("background", grid, true, true).
		AddPage("modal", modal, true, false)

	ui.app.SetRoot(pages, true)

	return ui
}

func (ui *UI) StartUILoop() {
	if err := ui.app.Run(); err != nil {
		panic(err)
	}
}

func (ui *UI) ShowDir(dirPath string) {
	ui.currentDir = path.Clean(path.Join(ui.currentDir, dirPath))

	dir := processDir(ui.currentDir)

	ui.dirContent.Clear()

	ui.header.SetText(dir.name)

	rowIndex := 0
	if ui.currentDir != ui.topDir {
		ui.dirContent.SetCell(0, 0, tview.NewTableCell(".."))
		rowIndex += 1
	}

	for _, item := range dir.files {
		ui.dirContent.SetCell(rowIndex, 0, tview.NewTableCell(item.name))
		rowIndex += 1
	}
}