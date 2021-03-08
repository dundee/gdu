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

	if ui.pages.HasPage("deleting") && key.Rune() != 'q' {
		return key
	}

	if key.Rune() == 'h' || key.Key() == tcell.KeyLeft {
		ui.handleLeft()
		return key
	}

	if key.Rune() == 'l' || key.Key() == tcell.KeyRight {
		ui.handleRight()
		return key
	}

	switch key.Rune() {
	case 'q':
		ui.app.Stop()
		return nil
	case '?':
		ui.showHelp()
	case 'd':
		ui.handleDelete()
	case 'v':
		ui.showFile()
	case 'a':
		ui.showApparentSize = !ui.showApparentSize
		if ui.currentDir != nil {
			ui.showDir()
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
	}
	return key
}

func (ui *UI) handleLeft() {
	if ui.pages.HasPage("confirm") {
		return
	}

	if ui.currentDirPath == ui.topDirPath {
		return
	}
	if ui.currentDir != nil {
		subDir := ui.currentDir
		ui.fileItemSelected(0, 0)
		index, _ := ui.currentDir.Files.IndexOf(subDir)
		if ui.currentDir != ui.topDir {
			index++
		}
		ui.table.Select(index, 0)
	}
}

func (ui *UI) handleRight() {
	if ui.pages.HasPage("confirm") || ui.pages.HasPage("progress") {
		return
	}

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
