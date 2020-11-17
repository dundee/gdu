package main

import (
	"github.com/rivo/tview"
)


type UI struct {
	app *tview.Application
	header *tview.TextView
	dirContent *tview.Table
}

func CreateUI() *UI {
	ui := &UI{}

	ui.app = tview.NewApplication()

	ui.header = tview.NewTextView()
	ui.header.SetText("aaa")

	ui.dirContent = tview.NewTable().SetSelectable(true, true)
	ui.dirContent.SetCell(0, 0, tview.NewTableCell("aa"))
	ui.dirContent.SetCell(1, 0, tview.NewTableCell("bb"))

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

func StartUILoop(ui *UI) {
	if err := ui.app.Run(); err != nil {
		panic(err)
	}
}

func ShowDir(ui *UI, dir Dir) {
	ui.dirContent.Clear()

	ui.header.SetText(dir.name)

	for i, item := range dir.files {
		ui.dirContent.SetCell(i, 0, tview.NewTableCell(item.name))
	}
}