package tui

import (
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

func (ui *UI) queueForDeletion(targets []deleteTarget, action DeleteAction, deleter itemDeleter) {
	ui.markQueuedForDeletion(targets)

	go func() {
		for _, target := range targets {
			ui.deleteQueue <- deleteQueueItem{target: target, action: action, deleter: deleter}
		}
	}()

	ui.markedItems = make(map[fs.Item]struct{})
	ui.refreshListing()
}

// markQueuedForDeletion remembers the items scheduled for background deletion,
// so the listing can show them dimmed until the deletion finishes.
func (ui *UI) markQueuedForDeletion(targets []deleteTarget) {
	ui.queuedMut.Lock()
	defer ui.queuedMut.Unlock()

	if ui.queuedDeletions == nil {
		ui.queuedDeletions = make(map[string]struct{}, len(targets))
	}
	for _, target := range targets {
		ui.queuedDeletions[target.item.GetPath()] = struct{}{}
	}
}

func (ui *UI) unmarkQueuedForDeletion(item fs.Item) {
	ui.queuedMut.Lock()
	defer ui.queuedMut.Unlock()
	delete(ui.queuedDeletions, item.GetPath())
}

func (ui *UI) queuedBackgroundPaths() map[string]struct{} {
	ui.queuedMut.Lock()
	defer ui.queuedMut.Unlock()

	if len(ui.queuedDeletions) == 0 {
		return nil
	}
	paths := make(map[string]struct{}, len(ui.queuedDeletions))
	for path := range ui.queuedDeletions {
		paths[path] = struct{}{}
	}
	return paths
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
	target, action := job.target, job.action
	// Keep the item marked while it is being deleted so it stays dimmed, then
	// drop the mark whether the deletion succeeds or fails.
	defer ui.unmarkQueuedForDeletion(target.item)
	ui.increaseActiveWorkers()
	defer ui.decreaseActiveWorkers()

	if err := job.deleter.Delete(target.dir, target.item, action.mode()); err != nil {
		msg := "Can't " + action.Verb() + " " + tview.Escape(target.item.GetName())
		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage(action.Acting())
			// the item is no longer queued, so clear its dimmed colour
			ui.refreshListing()
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
	ui.activeWorkers++
	workers := ui.activeWorkers
	ui.workersMut.Unlock()

	// Update the popup right away instead of waiting for the next poll, so
	// even a short background deletion becomes visible. Pass the count seen
	// here: the deletion may finish before this update reaches the event loop.
	ui.app.QueueUpdateDraw(func() {
		ui.showDeletionStatus(workers)
	})
}

func (ui *UI) decreaseActiveWorkers() {
	ui.workersMut.Lock()
	ui.activeWorkers--
	workers := ui.activeWorkers
	ui.workersMut.Unlock()

	if workers > 0 {
		ui.app.QueueUpdateDraw(func() {
			ui.showDeletionStatus(workers)
		})
		return
	}

	// Keep the popup around for a moment so the user can notice it even when
	// the deletion was short.
	time.AfterFunc(deletionStatusLinger, func() {
		ui.app.QueueUpdateDraw(ui.updateDeletionStatus)
	})
}
