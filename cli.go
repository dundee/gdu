package main

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	app             *tview.Application
	header          *tview.TextView
	currentDirLabel *tview.TextView
	dirContent      *tview.Table
	topDir          string
	currentDir      string
}

func (ui *UI) ItemSelected(row, column int) {
	selectedDir := ui.dirContent.GetCell(row, column).Text
	ui.ShowDir(selectedDir)
}

func (ui *UI) KeyPressed(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == 'q' {
		ui.app.Stop()
		return nil
	}
	return key
}

func CreateUI(topDir string) *UI {
	ui := &UI{}
	ui.topDir, _ = filepath.Abs(topDir)

	ui.app = tview.NewApplication()

	ui.header = tview.NewTextView()
	ui.header.SetText("gdu ~ Use arrow keys to navigate, press ? for help")

	ui.currentDirLabel = tview.NewTextView()

	ui.dirContent = tview.NewTable().SetSelectable(true, true)
	ui.dirContent.SetSelectedFunc(ui.ItemSelected)
	ui.dirContent.SetInputCapture(ui.KeyPressed)

	footer := tview.NewTextView().SetText("footer")

	grid := tview.NewGrid().SetRows(1, 1, 0, 1).SetColumns(0)
	grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
		AddItem(ui.dirContent, 2, 0, 1, 1, 0, 0, true).
		AddItem(footer, 3, 0, 1, 1, 0, 0, false)

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
	ui.currentDir, _ = filepath.Abs(filepath.Join(ui.currentDir, dirPath))
	ui.currentDirLabel.SetText("--- " + ui.currentDir + " ---")

	dir := processDir(ui.currentDir)

	ui.dirContent.Clear()

	rowIndex := 0
	if ui.currentDir != ui.topDir {
		ui.dirContent.SetCellSimple(0, 0, "..")
		rowIndex++
	}

	for _, item := range dir.files {
		ui.dirContent.SetCellSimple(rowIndex, 0, fmt.Sprintf("%10s", formatSize(item.size))+" "+item.name)
		rowIndex++
	}
}

func formatSize(size int64) string {
	if size > 1e9 {
		return fmt.Sprintf("%.1f GiB", float64(size)/float64(1e9))
	} else if size > 1e6 {
		return fmt.Sprintf("%.1f MiB", float64(size)/float64(1e6))
	} else if size > 1e3 {
		return fmt.Sprintf("%.1f KiB", float64(size)/float64(1e3))
	}
	return fmt.Sprintf("%d B", size)
}
