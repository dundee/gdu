package tui

import (
	"github.com/dundee/gdu/v6/pkg/fs"
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
	defer ui.deletionsDone(1)

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

// addPendingDeletions counts n more items to delete.
func (ui *UI) addPendingDeletions(n int) {
	ui.workersMut.Lock()
	defer ui.workersMut.Unlock()
	ui.pendingDeletions += n
}

// deletionsDone counts n items as deleted, or given up, and quits when gdu
// is to quit once no deletion is left.
func (ui *UI) deletionsDone(n int) {
	ui.workersMut.Lock()
	ui.pendingDeletions -= n
	quit := ui.quitAfterDeletions
	if ui.pendingDeletions > 0 {
		quit = nil
	}
	ui.workersMut.Unlock()
	if quit != nil {
		ui.app.QueueUpdateDraw(quit)
	}
}

// pendingDeletionCount returns how many items are queued or being deleted.
func (ui *UI) pendingDeletionCount() int {
	ui.workersMut.Lock()
	defer ui.workersMut.Unlock()
	return ui.pendingDeletions
}

// quitAfterDeleting quits with quit once no deletion is left, at once when
// none is.
func (ui *UI) quitAfterDeleting(quit func()) {
	ui.workersMut.Lock()
	pending := ui.pendingDeletions
	if pending > 0 {
		ui.quitAfterDeletions = quit
	}
	ui.workersMut.Unlock()
	if pending == 0 {
		quit()
	}
}
