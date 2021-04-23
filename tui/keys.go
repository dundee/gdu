package tui

import (
	"github.com/dundee/gdu/v4/analyze"
	"github.com/gdamore/tcell/v2"
)

func (ui *UI) keyPressed(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() == tcell.KeyEsc || key.Rune() == 'q' {
		if ui.pages.HasPage("help") {
			ui.pages.RemovePage("help")
			_, page := ui.pages.GetFrontPage()
			ui.app.SetFocus(page)
			return nil
		}
		if ui.pages.HasPage("file") {
			return key // send event to primitive
		}
	}

	switch key.Rune() {
	case 'q':
		ui.app.Stop()
		return nil
	case '?':
		ui.showHelp()
	}

	if ui.pages.HasPage("confirm") || ui.pages.HasPage("progress") || ui.pages.HasPage("deleting") {
		return key
	}

	if key.Rune() == 'h' || key.Key() == tcell.KeyLeft {
		ui.handleLeft()
		return nil
	}

	if key.Rune() == 'l' || key.Key() == tcell.KeyRight {
		ui.handleRight()
		return nil
	}

	switch key.Rune() {
	case 'd':
		ui.handleDelete()
	case 'v':
		ui.showFile()
	case 'a':
		ui.ShowApparentSize = !ui.ShowApparentSize
		if ui.currentDir != nil {
			row, column := ui.table.GetSelection()
			ui.showDir()
			ui.table.Select(row, column)
		}
	case 'r':
		if ui.currentDir != nil {
			ui.rescanDir()
		}
	case 's':
		ui.setSorting("size")
	case 'c':
		ui.setSorting("itemCount")
	case 'n':
		ui.setSorting("name")
	default:
		return key
	}
	return nil
}

func (ui *UI) handleLeft() {
	if ui.currentDirPath == ui.topDirPath {
		return
	}
	if ui.currentDir != nil {
		ui.fileItemSelected(0, 0)
	}
}

func (ui *UI) handleRight() {
	row, column := ui.table.GetSelection()
	if ui.currentDirPath != ui.topDirPath && row == 0 { // do not select /..
		return
	}

	if ui.currentDir != nil {
		ui.fileItemSelected(row, column)
	} else {
		ui.deviceItemSelected(row, column)
	}
}

func (ui *UI) handleDelete() {
	if ui.currentDir == nil {
		return
	}

	// do not allow deleting parent dir
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(analyze.Item)
	if selectedFile == ui.currentDir.Parent {
		return
	}

	if ui.askBeforeDelete {
		ui.confirmDeletion()
	} else {
		ui.deleteSelected()
	}
}
