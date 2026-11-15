package tui

import (
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

func (ui *UI) queueForDeletion(targets []deleteTarget, action DeleteAction, deleter itemDeleter) {
	go func() {
		for _, target := range targets {
			ui.deleteQueue <- deleteQueueItem{target: target, action: action, deleter: deleter}
		}
	}()

	ui.markedItems = make(map[fs.Item]struct{})
}

func (ui *UI) deleteWorker() {
	defer func() {
		if r := recover(); r != nil {
			ui.app.Stop()
			panic(r)
		}
	}()

	for job := range ui.deleteQueue {
		ui.deleteItem(job)
	}
}

func (ui *UI) deleteItem(job deleteQueueItem) {
	ui.increaseActiveWorkers()
	defer ui.decreaseActiveWorkers()

	target, action := job.target, job.action
	if err := job.deleter.Delete(target.dir, target.item, action.mode()); err != nil {
		msg := "Can't " + action.Verb() + " " + tview.Escape(target.item.GetName())
		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage(action.Acting())
			ui.showErr(msg, err)
		})
		if ui.done != nil {
			ui.done <- struct{}{}
		}
		return
	}

	dirPath := target.dir.GetPath()
	ui.app.QueueUpdateDraw(func() {
		if ui.currentDir == nil || ui.currentDir.GetPath() != dirPath {
			return
		}
		row, _ := ui.table.GetSelection()
		x, y := ui.table.GetOffset()
		ui.showDir()
		ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
		ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
	})
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
