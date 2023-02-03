package tui

import (
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) onMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if event == nil {
		return nil, action
	}

	if ui.pages.HasPage("confirm") ||
		ui.pages.HasPage("progress") ||
		ui.pages.HasPage("deleting") ||
		ui.pages.HasPage("emptying") ||
		ui.pages.HasPage("help") {
		return nil, action
	}

	switch action {
	case tview.MouseLeftDoubleClick:
		row, column := ui.table.GetSelection()
		if ui.currentDirPath != ui.topDirPath && row == 0 {
			ui.handleLeft()
		} else {
			selectedFile := ui.table.GetCell(row, column).GetReference().(fs.Item)
			if selectedFile.IsDir() {
				ui.handleRight()
			} else {
				ui.showFile()
			}
		}
		return nil, action
	case tview.MouseScrollUp:
		fallthrough
	case tview.MouseScrollDown:
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
