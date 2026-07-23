package tui

import (
	"strconv"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) fileItemMarked(row int) {
	if _, ok := ui.markedRows[row]; ok {
		delete(ui.markedRows, row)
	} else {
		ui.markedRows[row] = struct{}{}
	}
	ui.showDir()
	// select next row if possible
	ui.table.Select(min(row+1, ui.table.GetRowCount()-1), 0)
}

func (ui *UI) deleteMarked(action DeleteAction) {
	acting := action.Acting()

	var currentDir fs.Item
	var markedItems []fs.Item
	for row := range ui.markedRows {
		item := ui.table.GetCell(row, 0).GetReference().(fs.Item)
		markedItems = append(markedItems, item)
	}

	if ui.deleteInBackground {
		ui.queueForDeletion(markedItems, action)
		return
	}

	modal := tview.NewModal()
	ui.pages.AddPage(acting, modal, true, true)

	currentRow, _ := ui.table.GetSelection()

	var deleteFun func(fs.Item, fs.Item) error

	go func() {
		for _, one := range markedItems {
			ui.app.QueueUpdateDraw(func() {
				modal.SetText(
					cases.Title(language.English).String(acting) +
						" " +
						tview.Escape(one.GetName()) +
						"...",
				)
			})

			switch action {
			case ActionEmpty:
				if !one.IsDir() {
					deleteFun = ui.emptier
				} else {
					deleteFun = ui.remover
				}
			case ActionMoveToTrash:
				deleteFun = ui.trasher
			default:
				deleteFun = ui.remover
			}

			var deleteItems []fs.Item
			if action == ActionEmpty && one.IsDir() {
				currentDir = one
				for file := range currentDir.GetFiles(fs.SortBySize, fs.SortDesc) {
					deleteItems = append(deleteItems, file)
				}
			} else {
				currentDir = ui.currentDir
				deleteItems = append(deleteItems, one)
			}

			for _, item := range deleteItems {
				if err := deleteFun(currentDir, item); err != nil {
					msg := "Can't " + action.Verb() + " " + tview.Escape(one.GetName())
					ui.app.QueueUpdateDraw(func() {
						ui.pages.RemovePage(acting)
						ui.showErr(msg, err)
					})
					if ui.done != nil {
						ui.done <- struct{}{}
					}
					return
				}
			}
		}

		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage(acting)
			ui.pages.RemovePage(acting)
			ui.markedRows = make(map[int]struct{})
			x, y := ui.table.GetOffset()
			ui.showDir()
			ui.table.Select(min(currentRow, ui.table.GetRowCount()-1), 0)
			ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
}

func (ui *UI) confirmDeletionMarked(action DeleteAction) {
	modal := tview.NewModal().
		SetText(
			"Are you sure you want to " +
				action.Verb() + " [::b]" +
				strconv.Itoa(len(ui.markedRows)) +
				"[::-] items?",
		).
		AddButtons([]string{"no", "yes", "don't ask me again"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 2:
				ui.askBeforeDelete = false
				fallthrough
			case 1:
				ui.deleteMarked(action)
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
	if len(ui.markedRows) == 0 {
		return
	}
	for row := range ui.markedRows {
		item := ui.table.GetCell(row, 0).GetReference().(fs.Item)
		ui.markedPaths = append(ui.markedPaths, item.GetPath())
	}
	ui.markedRows = make(map[int]struct{})
	selectRow, _ := ui.table.GetSelection()
	ui.showDir()
	ui.table.Select(selectRow, 0)
}
