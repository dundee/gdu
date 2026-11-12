package tui

import (
	"strconv"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) fileItemMarked(row int) {
	if item, ok := ui.rowItem(row, 0); ok {
		if _, marked := ui.markedItems[item]; marked {
			delete(ui.markedItems, item)
		} else {
			ui.markedItems[item] = struct{}{}
		}
	}
	ui.showDir()
	// select next row if possible
	ui.table.Select(min(row+1, ui.table.GetRowCount()-1), 0)
}

func (ui *UI) confirmDeletionMarked(targets []deleteTarget, action DeleteAction) {
	modal := tview.NewModal().
		SetText(
			"Are you sure you want to " +
				action.Verb() + " [::b]" +
				strconv.Itoa(len(targets)) +
				"[::-] items?",
		).
		AddButtons([]string{No, Yes, DontAskAgain}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 2:
				ui.askBeforeDelete = false
				fallthrough
			case 1:
				ui.deleteTargets(targets, action)
			}
			ui.pages.RemovePage("confirm")
		})

	if !ui.UseColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	} else {
		modal.SetBackgroundColor(tcell.ColorBlack)
	}
	modal.SetBorderColor(tcell.ColorDefault)

	ui.pages.AddPage("confirm", modal, true, true)
}

func (ui *UI) printMarked() {
	if len(ui.markedItems) == 0 {
		return
	}
	for _, item := range ui.markedListedItems() {
		ui.markedPaths = append(ui.markedPaths, item.GetPath())
	}
	ui.markedItems = make(map[fs.Item]struct{})
	selectRow, _ := ui.table.GetSelection()
	ui.showDir()
	ui.table.Select(selectRow, 0)
}
