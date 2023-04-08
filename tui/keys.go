package tui

import (
	"fmt"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
)

func (ui *UI) keyPressed(key *tcell.EventKey) *tcell.EventKey {
	if ui.pages.HasPage("file") {
		return key // send event to primitive
	}
	if ui.filtering {
		return key
	}

	key = ui.handleClosingModals(key)
	if key == nil {
		return nil
	}
	key = ui.handleInfoPageEvents(key)
	if key == nil {
		return nil
	}
	key = ui.handleQuit(key)
	if key == nil {
		return nil
	}

	if ui.pages.HasPage("confirm") ||
		ui.pages.HasPage("progress") ||
		ui.pages.HasPage("deleting") ||
		ui.pages.HasPage("emptying") {
		return key
	}

	key = ui.handleHelp(key)
	if key == nil {
		return nil
	}

	if ui.pages.HasPage("help") {
		return key
	}

	key = ui.handleShell(key)
	if key == nil {
		return nil
	}

	key = ui.handleLeftRight(key)
	if key == nil {
		return nil
	}

	key = ui.handleFiltering(key)
	if key == nil {
		return nil
	}

	return ui.handleMainActions(key)
}

func (ui *UI) handleClosingModals(key *tcell.EventKey) *tcell.EventKey {
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
	return key
}

func (ui *UI) handleInfoPageEvents(key *tcell.EventKey) *tcell.EventKey {
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
		ui.showInfo() // refresh file info after any change
	}
	return key
}

func (ui *UI) handleQuit(key *tcell.EventKey) *tcell.EventKey {
	switch key.Rune() {
	case 'Q':
		ui.app.Stop()
		fmt.Fprintf(ui.output, "%s\n", ui.currentDirPath)
		return nil
	case 'q':
		ui.app.Stop()
		return nil
	}
	return key
}

func (ui *UI) handleHelp(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == '?' {
		if ui.pages.HasPage("help") {
			ui.pages.RemovePage("help")
			ui.app.SetFocus(ui.table)
			return nil
		}
		ui.showHelp()
		return nil
	}
	return key
}

func (ui *UI) handleShell(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == 'b' {
		ui.spawnShell()
		return nil
	}
	return key
}

func (ui *UI) handleLeftRight(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == 'h' || key.Key() == tcell.KeyLeft {
		ui.handleLeft()
		return nil
	}

	if key.Rune() == 'l' || key.Key() == tcell.KeyRight {
		ui.handleRight()
		return nil
	}
	return key
}

func (ui *UI) handleFiltering(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() == tcell.KeyTab && ui.filteringInput != nil {
		ui.filtering = true
		ui.app.SetFocus(ui.filteringInput)
		return nil
	}
	return key
}

func (ui *UI) handleMainActions(key *tcell.EventKey) *tcell.EventKey {
	switch key.Rune() {
	case 'd':
		ui.handleDelete(false)
	case 'e':
		ui.handleDelete(true)
	case 'v':
		ui.showFile()
	case 'o':
		ui.openItem()
	case 'i':
		ui.showInfo()
	case 'a':
		ui.ShowApparentSize = !ui.ShowApparentSize
		if ui.currentDir != nil {
			row, column := ui.table.GetSelection()
			ui.showDir()
			ui.table.Select(row, column)
		}
	case 'B':
		ui.ShowRelativeSize = !ui.ShowRelativeSize
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
		return nil
	case ' ':
		ui.handleMark()
	}
	return key
}

func (ui *UI) handleLeft() {
	if ui.currentDirPath == ui.topDirPath {
		if ui.devices != nil {
			ui.currentDir = nil
			err := ui.ListDevices(ui.getter)
			if err != nil {
				ui.showErr("Error listing devices", err)
			}
		}
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
	selectedFile, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedFile == ui.currentDir.GetParent() {
		return
	}

	if ui.askBeforeDelete {
		ui.confirmDeletion(shouldEmpty)
	} else {
		ui.delete(shouldEmpty)
	}
}

func (ui *UI) handleMark() {
	if ui.currentDir == nil {
		return
	}
	// do not allow deleting parent dir
	row, column := ui.table.GetSelection()
	selectedFile, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedFile == ui.currentDir.GetParent() {
		return
	}

	ui.fileItemMarked(row)
}
