package tui

import (
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

func (ui *UI) queueForDeletion(items []fs.Item, shouldEmpty bool) {
	go func() {
		for _, item := range items {
			ui.deleteQueue <- deleteQueueItem{item: item, shouldEmpty: shouldEmpty}
		}
	}()

	ui.markedRows = make(map[int]struct{})
}

func (ui *UI) deleteWorker() {
	defer func() {
		if r := recover(); r != nil {
			ui.app.Stop()
			panic(r)
		}
	}()

	for item := range ui.deleteQueue {
		ui.deleteItem(item.item, item.shouldEmpty)
	}
}

func (ui *UI) deleteItem(item fs.Item, shouldEmpty bool) {
	ui.increaseActiveWorkers()
	defer ui.decreaseActiveWorkers()

	var action, acting string
	if shouldEmpty {
		action = actionEmpty
	} else {
		action = actionDelete
	}

	var deleteFun func(fs.Item, fs.Item) error
	if shouldEmpty && !item.IsDir() {
		deleteFun = ui.emptier
	} else {
		deleteFun = ui.remover
	}

	var parentDir fs.Item
	var deleteItems []fs.Item
	if shouldEmpty && item.IsDir() {
		parentDir = item.(*analyze.Dir)
		for _, file := range item.GetFilesLocked() {
			deleteItems = append(deleteItems, file)
		}
	} else {
		parentDir = ui.currentDir
		deleteItems = append(deleteItems, item)
	}

	for _, toDelete := range deleteItems {
		if err := deleteFun(parentDir, toDelete); err != nil {
			msg := "Can't " + action + " " + tview.Escape(toDelete.GetName())
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

	if item.GetParent().GetPath() == ui.currentDir.GetPath() {
		ui.app.QueueUpdateDraw(func() {
			row, _ := ui.table.GetSelection()
			x, y := ui.table.GetOffset()
			ui.showDir()
			ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
			ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
		})
	}
	if ui.done != nil {
		ui.done <- struct{}{}
	}
}

func (ui *UI) increaseActiveWorkers() {
	ui.workersMut.Lock()
	defer ui.workersMut.Unlock()
	ui.activeWorkers++
}

func (ui *UI) decreaseActiveWorkers() {
	ui.workersMut.Lock()
	defer ui.workersMut.Unlock()
	ui.activeWorkers--
}
