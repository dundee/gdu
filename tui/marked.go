package tui

import (
	"strconv"
	"strings"

	"github.com/dundee/gdu/v5/pkg/analyze"
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

func (ui *UI) deleteMarked(shouldEmpty bool) {
	var action, acting string
	if shouldEmpty {
		action = "empty "
		acting = "emptying"
	} else {
		action = "delete "
		acting = "deleting"
	}

	modal := tview.NewModal()
	ui.pages.AddPage(acting, modal, true, true)

	var currentDir fs.Item
	var markedItems []fs.Item
	for row := range ui.markedRows {
		item := ui.table.GetCell(row, 0).GetReference().(fs.Item)
		markedItems = append(markedItems, item)
	}

	currentRow, _ := ui.table.GetSelection()

	var deleteFun func(fs.Item, fs.Item) error

	go func() {
		for _, one := range markedItems {

			ui.app.QueueUpdateDraw(func() {
				modal.SetText(
					// nolint: staticcheck // Why: fixed string
					strings.Title(acting) +
						" " +
						tview.Escape(one.GetName()) +
						"...",
				)
			})

			if shouldEmpty && !one.IsDir() {
				deleteFun = ui.emptier
			} else {
				deleteFun = ui.remover
			}

			var deleteItems []fs.Item
			if shouldEmpty && one.IsDir() {
				currentDir = one.(*analyze.Dir)
				for _, file := range currentDir.GetFiles() {
					deleteItems = append(deleteItems, file)
				}
			} else {
				currentDir = ui.currentDir
				deleteItems = append(deleteItems, one)
			}

			for _, item := range deleteItems {
				if err := deleteFun(currentDir, item); err != nil {
					msg := "Can't " + action + tview.Escape(one.GetName())
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
			ui.markedRows = make(map[int]struct{})
			ui.showDir()
			ui.table.Select(min(currentRow, ui.table.GetRowCount()-1), 0)
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
}

func (ui *UI) confirmDeletionMarked(shouldEmpty bool) {
	var action string
	if shouldEmpty {
		action = "empty"
	} else {
		action = "delete"
	}

	modal := tview.NewModal().
		SetText(
			"Are you sure you want to " +
				action + " [::b]" +
				strconv.Itoa(len(ui.markedRows)) +
				"[::-] items?",
		).
		AddButtons([]string{"yes", "no", "don't ask me again"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 2:
				ui.askBeforeDelete = false
				fallthrough
			case 0:
				ui.deleteMarked(shouldEmpty)
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
