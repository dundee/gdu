package tui

import (
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) onMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if event == nil {
		return nil, action
	}

	// only the listing takes the mouse: not a dialog, the help or the file
	// viewer in front of it
	if front, _ := ui.pages.GetFrontPage(); front != "background" {
		return nil, action
	}

	// nolint: exhaustive // Why: we don't need to handle all mouse events
	switch action {
	case tview.MouseLeftDoubleClick:
		row, column := ui.table.GetSelection()
		if ui.currentDir == nil {
			// the list of devices
			ui.deviceItemSelected(row, column)
			return nil, action
		}
		if ui.currentDirPath != ui.topDirPath && row == 0 {
			ui.handleLeft()
			return nil, action
		}
		cell := ui.table.GetCell(row, column)
		selectedFile, ok := cell.GetReference().(fs.Item)
		if !ok || selectedFile == nil {
			// an empty directory has no row to open
			return nil, action
		}
		if selectedFile.IsDir() {
			ui.handleRight()
		} else {
			if ui.noViewFile {
				ui.showMessage(" Viewing files is disabled!")
				return nil, action
			}
			ui.showFile()
		}
		return nil, action
	case tview.MouseScrollUp, tview.MouseScrollDown:
		row, column := ui.table.GetSelection()
		if action == tview.MouseScrollUp && row > 0 {
			row--
		} else if action == tview.MouseScrollDown && row+1 < ui.table.GetRowCount() {
			row++
		}
		ui.table.Select(row, column)
		return nil, action
	}

	return event, action
}
