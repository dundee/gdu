package tui

import (
	"fmt"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/gdamore/tcell/v2"
)

func (ui *UI) keyPressed(key *tcell.EventKey) *tcell.EventKey {
	if ui.pages.HasPage("file") {
		return key // send event to primitive
	}
	if ui.filtering {
		return key
	}

	if key.Key() == tcell.KeyEsc || key.Rune() == 'q' {
		if ui.pages.HasPage("help") {
			ui.pages.RemovePage("help")
			ui.app.SetFocus(ui.table)
			return nil
		}
		if ui.pages.HasPage("info") {
			ui.pages.RemovePage("info")
			ui.app.SetFocus(ui.table)
			return nil
		}
	}

	if ui.pages.HasPage("info") {
		switch key.Rune() {
		case 'i':
			ui.pages.RemovePage("info")
			ui.app.SetFocus(ui.table)
			return nil
		case '?':
			return nil
		}

		if key.Key() == tcell.KeyUp ||
			key.Key() == tcell.KeyDown ||
			key.Rune() == 'j' ||
			key.Rune() == 'k' {
			row, column := ui.table.GetSelection()
			if (key.Key() == tcell.KeyUp || key.Rune() == 'k') && row > 0 {
				row--
			} else if (key.Key() == tcell.KeyDown || key.Rune() == 'j') &&
				row+1 < ui.table.GetRowCount() {
				row++
			}
			ui.table.Select(row, column)
		}
		defer ui.showInfo() // refresh file info after any change
	}

	switch key.Rune() {
	case 'Q':
		ui.app.Stop()
		fmt.Fprintf(ui.output, "%s\n", ui.currentDirPath)
		return nil
	case 'q':
		ui.app.Stop()
		return nil
	case 'b':
		ui.spawnShell()
		return nil
	case '?':
		if ui.pages.HasPage("help") {
			ui.pages.RemovePage("help")
			ui.app.SetFocus(ui.table)
			return nil
		}
		ui.showHelp()
	}

	if ui.pages.HasPage("confirm") ||
		ui.pages.HasPage("progress") ||
		ui.pages.HasPage("deleting") ||
		ui.pages.HasPage("emptying") ||
		ui.pages.HasPage("help") {
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

	if key.Key() == tcell.KeyTab && ui.filteringInput != nil {
		ui.filtering = true
		ui.app.SetFocus(ui.filteringInput)
		return nil
	}

	switch key.Rune() {
	case 'd':
		ui.handleDelete(false)
	case 'e':
		ui.handleDelete(true)
	case 'v':
		ui.showFile()
	case 'i':
		ui.showInfo()
	case 'a':
		ui.ShowApparentSize = !ui.ShowApparentSize
		if ui.currentDir != nil {
			row, column := ui.table.GetSelection()
			ui.showDir()
			ui.table.Select(row, column)
		}
	case 'c':
		ui.showItemCount = !ui.showItemCount
		if ui.currentDir != nil {
			row, column := ui.table.GetSelection()
			ui.showDir()
			ui.table.Select(row, column)
		}
	case 'm':
		ui.showMtime = !ui.showMtime
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
	case 'C':
		ui.setSorting("itemCount")
	case 'n':
		ui.setSorting("name")
	case 'M':
		ui.setSorting("mtime")
	case '/':
		ui.showFilterInput()
	}
	return key
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

func (ui *UI) handleDelete(shouldEmpty bool) {
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
		ui.confirmDeletion(shouldEmpty)
	} else {
		ui.deleteSelected(shouldEmpty)
	}
}
