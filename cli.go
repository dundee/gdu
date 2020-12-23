package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	app             *tview.Application
	header          *tview.TextView
	footer          *tview.TextView
	currentDirLabel *tview.TextView
	pages           *tview.Pages
	modal           *tview.Modal
	dirContent      *tview.Table
	currentDir      *File
	topDirPath      string
	currentDirPath  string
}

func (ui *UI) ItemSelected(row, column int) {
	selectedDir := ui.dirContent.GetCell(row, column).GetReference().(*File)
	if !selectedDir.isDir {
		return
	}

	ui.currentDir = selectedDir
	ui.ShowDir()
}

func (ui *UI) KeyPressed(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == 'q' {
		ui.app.Stop()
		return nil
	}
	return key
}

func CreateUI(topDirPath string) *UI {
	ui := &UI{}
	ui.topDirPath, _ = filepath.Abs(topDirPath)

	ui.app = tview.NewApplication()
	ui.app.SetInputCapture(ui.KeyPressed)

	ui.header = tview.NewTextView()
	ui.header.SetText("gdu ~ Use arrow keys to navigate, press ? for help")
	ui.header.SetTextColor(tcell.ColorBlack)
	ui.header.SetBackgroundColor(tcell.ColorWhite)

	ui.currentDirLabel = tview.NewTextView()

	ui.dirContent = tview.NewTable().SetSelectable(true, true)
	ui.dirContent.SetSelectedFunc(ui.ItemSelected)

	ui.footer = tview.NewTextView()
	ui.footer.SetTextColor(tcell.ColorBlack)
	ui.footer.SetBackgroundColor(tcell.ColorWhite)
	ui.footer.SetText("No items to diplay.")

	grid := tview.NewGrid().SetRows(1, 1, 0, 1).SetColumns(0)
	grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
		AddItem(ui.dirContent, 2, 0, 1, 1, 0, 0, true).
		AddItem(ui.footer, 3, 0, 1, 1, 0, 0, false)

	ui.modal = tview.NewModal().SetText("Scanning...")

	ui.pages = tview.NewPages().
		AddPage("background", grid, true, true).
		AddPage("modal", ui.modal, true, true)

	ui.app.SetRoot(ui.pages, true)

	return ui
}

func (ui *UI) StartUILoop() {
	if err := ui.app.Run(); err != nil {
		panic(err)
	}
}

func (ui *UI) updateProgress(statusChannel chan CurrentProgress) {
	for {
		progress := <-statusChannel

		if progress.done {
			return
		}

		ui.app.QueueUpdateDraw(func() {
			ui.modal.SetText("Current item: " + progress.currentItemName)
		})

		time.Sleep(100 * time.Millisecond)
	}
}

func (ui *UI) ShowDir() {
	ui.currentDirPath = ui.currentDir.path
	ui.currentDirLabel.SetText("--- " + ui.currentDirPath + " ---")

	ui.dirContent.Clear()

	rowIndex := 0
	if ui.currentDirPath != ui.topDirPath {
		cell := tview.NewTableCell("           /..")
		cell.SetReference(ui.currentDir.parent)
		ui.dirContent.SetCell(0, 0, cell)
		rowIndex++
	}

	for i, item := range ui.currentDir.files {
		cell := tview.NewTableCell(formatRow(item))
		cell.SetReference(ui.currentDir.files[i])
		ui.dirContent.SetCell(rowIndex, 0, cell)
		rowIndex++
	}

	ui.footer.SetText("Apparent size: " + formatSize(ui.currentDir.size) + " Items: " + fmt.Sprint(ui.currentDir.itemCount))
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

func formatRow(item *File) string {
	part := int(float64(item.size) / float64(item.parent.size) * 10.0)
	row := fmt.Sprintf("%10s", formatSize(item.size))
	row += " ["
	for i := 0; i < 10; i++ {
		if part > i {
			row += "#"
		} else {
			row += " "
		}
	}
	row += "] "

	if item.isDir {
		row += "/"
	}
	row += item.name
	return row
}
