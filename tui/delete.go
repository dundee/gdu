package tui

import (
	"path/filepath"
	"time"

	"github.com/rivo/tview"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/pkg/remove"
)

// itemDeleter changes the filesystem and the tree for a confirmed deletion.
// remove.Deleter implements it.
type itemDeleter interface {
	Delete(dir, item fs.Item, mode remove.Mode) error
	Check(dir, item fs.Item, mode remove.Mode) error
}

// deleteTarget is an item the user chose to delete together with the
// directory it is removed from, both captured when the choice was made. Later
// changes to the selection or to the listing cannot change what is deleted.
type deleteTarget struct {
	dir  fs.Item
	item fs.Item
}

func (a DeleteAction) mode() remove.Mode {
	switch a {
	case ActionEmpty:
		return remove.ModeEmpty
	case ActionMoveToTrash:
		return remove.ModeTrash
	case ActionDelete:
		return remove.ModeRemove
	}
	return remove.ModeRemove
}

func (ui *UI) defaultDeleter(roots []string) itemDeleter {
	opts := []remove.Option{remove.WithTrash(ui.trasher)}
	if ui.deleteInParallel {
		opts = append(opts, remove.WithParallel())
	}
	return remove.NewDeleter(roots, opts...)
}

// deleter returns a deleter confined to the scanned roots of the tree shown.
func (ui *UI) deleter() itemDeleter {
	return ui.newDeleter(ui.session.RootPaths())
}

// rowItem returns the item listed on row. ok is false for the "/.." row,
// which stands for the parent directory rather than an entry, and for rows
// without an item.
func (ui *UI) rowItem(row, column int) (item fs.Item, ok bool) {
	if ui.parentRowShown && row == 0 {
		return nil, false
	}
	cell := ui.table.GetCell(row, column)
	if cell == nil {
		return nil, false
	}
	item, ok = cell.GetReference().(fs.Item)
	return item, ok && item != nil
}

func (ui *UI) selectedRowItem() (fs.Item, bool) {
	row, column := ui.table.GetSelection()
	return ui.rowItem(row, column)
}

// listedItems returns the items listed in the current directory, in the order
// they are shown.
func (ui *UI) listedItems() []fs.Item {
	var items []fs.Item
	for row := range ui.table.GetRowCount() {
		if item, ok := ui.rowItem(row, 0); ok {
			items = append(items, item)
		}
	}
	return items
}

// markedListedItems returns the marked items, in the order they are shown.
func (ui *UI) markedListedItems() []fs.Item {
	var items []fs.Item
	for _, item := range ui.listedItems() {
		if _, marked := ui.markedItems[item]; marked {
			items = append(items, item)
		}
	}
	return items
}

// targetFor captures item, listed in the current directory, as a deletion
// target. A collapsed row (--collapse-path) stands for a directory nested
// deeper, which is removed from its own parent.
func (ui *UI) targetFor(item fs.Item) deleteTarget {
	dir := ui.currentDir
	dirPath, itemPath := dir.GetPath(), item.GetPath()
	if filepath.Join(dirPath, item.GetName()) != itemPath {
		if rel, err := filepath.Rel(dirPath, itemPath); err == nil && filepath.IsLocal(rel) {
			dir = item.GetParent()
		}
	}
	return deleteTarget{dir: dir, item: item}
}

func (ui *UI) targetsFor(items []fs.Item) []deleteTarget {
	targets := make([]deleteTarget, 0, len(items))
	for _, item := range items {
		targets = append(targets, ui.targetFor(item))
	}
	return targets
}

// selectedTargets captures the marked items, or the selected one when nothing
// is marked.
func (ui *UI) selectedTargets() []deleteTarget {
	if len(ui.markedItems) > 0 {
		return ui.targetsFor(ui.markedListedItems())
	}
	item, ok := ui.selectedRowItem()
	if !ok {
		return nil
	}
	return []deleteTarget{ui.targetFor(item)}
}

// checkTargets shows why the first target that cannot be deleted is refused.
func (ui *UI) checkTargets(deleter itemDeleter, targets []deleteTarget, action DeleteAction) bool {
	for _, target := range targets {
		if err := deleter.Check(target.dir, target.item, action.mode()); err != nil {
			ui.showErr("Can't "+action.Verb()+" "+tview.Escape(target.item.GetName()), err)
			return false
		}
	}
	return true
}

// deleteTargets deletes targets, in the background when configured so. It is
// the only way the TUI changes the filesystem.
func (ui *UI) deleteTargets(targets []deleteTarget, action DeleteAction) {
	if len(targets) == 0 {
		return
	}
	deleter := ui.deleter()

	if ui.deleteInBackground {
		ui.queueForDeletion(targets, action, deleter)
		return
	}

	items := make([]deleteQueueItem, 0, len(targets))
	for _, target := range targets {
		items = append(items, deleteQueueItem{target: target, action: action, deleter: deleter})
	}
	ui.queueForegroundDeletion(items)
	ui.markedItems = make(map[fs.Item]struct{})

	// deleteTargets is called from the event loop (a key handler or a modal
	// callback), so the listing must be refreshed directly. Queuing a draw from
	// here would wait for the very loop that is running this code and freeze
	// the UI. Refreshing also dims the item that is about to be deleted.
	ui.refreshListing()
	ui.updateDeletionStatus()
}

// activeDeletionJob returns the running foreground deletion, if any.
func (ui *UI) activeDeletionJob() *deletionJob {
	ui.deletionMut.RLock()
	defer ui.deletionMut.RUnlock()
	return ui.activeDeletion
}

// dimmedDeletionPaths returns the paths of the items that are being deleted or
// waiting in a queue, so the listing can show them dimmed.
func (ui *UI) dimmedDeletionPaths() map[string]struct{} {
	paths := ui.queuedBackgroundPaths()
	job := ui.activeDeletionJob()
	if job == nil {
		return paths
	}
	for path := range job.dimmedPaths() {
		if paths == nil {
			paths = make(map[string]struct{})
		}
		paths[path] = struct{}{}
	}
	return paths
}

// queueForegroundDeletion starts a foreground deletion job, or appends to the
// running one. A job is running while its dialog is shown or hidden; appending
// is what lets the user queue more deletions while browsing.
func (ui *UI) queueForegroundDeletion(items []deleteQueueItem) {
	ui.deletionMut.Lock()
	job := ui.activeDeletion
	if job == nil {
		first := items[0]
		acting := first.action.Acting()
		title := cases.Title(language.English).String(acting)
		modal := tview.NewModal().SetText(deletionModalText(title, first.target.item.GetName()))
		progress := &deletionProgress{page: acting, modal: modal}
		job = &deletionJob{ui: ui, progress: progress}
		ui.activeDeletion = job
		ui.pages.AddPage(acting, modal, true, true)
	}
	ui.deletionMut.Unlock()

	job.add(items)
	ui.updateDeletionStatus()
}

func deletionModalText(title, name string) string {
	return title + " " + tview.Escape(name) + "...\n\nPress Ctrl+B to hide this dialog and keep browsing."
}

// add appends items to the job and starts the worker when it is idle.
func (job *deletionJob) add(items []deleteQueueItem) {
	job.mut.Lock()
	defer job.mut.Unlock()

	if job.closed {
		return
	}
	job.pending = append(job.pending, items...)
	if !job.running {
		job.running = true
		go job.run()
	}
}

// take returns the next item to delete. When the queue is empty it marks the
// worker idle so a later add can start it again.
func (job *deletionJob) take() (deleteQueueItem, bool) {
	job.mut.Lock()
	defer job.mut.Unlock()

	if len(job.pending) == 0 {
		job.running = false
		return deleteQueueItem{}, false
	}
	item := job.pending[0]
	job.pending = job.pending[1:]
	job.currentPath = item.target.item.GetPath()
	return item, true
}

func (job *deletionJob) run() {
	for {
		item, ok := job.take()
		if !ok {
			job.ui.app.QueueUpdateDraw(job.finishIfIdle)
			if job.ui.done != nil {
				job.ui.done <- struct{}{}
			}
			return
		}

		name := tview.Escape(item.target.item.GetName())
		title := cases.Title(language.English).String(item.action.Acting())
		job.ui.app.QueueUpdateDraw(func() {
			job.progress.modal.SetText(deletionModalText(title, name))
			job.ui.updateDeletionStatus()
		})

		if err := item.deleter.Delete(item.target.dir, item.target.item, item.action.mode()); err != nil {
			msg := "Can't " + item.action.Verb() + " " + name
			job.ui.app.QueueUpdateDraw(func() {
				job.fail()
				job.ui.showErr(msg, err)
			})
			if job.ui.done != nil {
				job.ui.done <- struct{}{}
			}
			return
		}

		job.mut.Lock()
		job.currentPath = ""
		job.mut.Unlock()
	}
}

// dimmedPaths returns the paths of the items this job is deleting or waiting
// to delete. They are shown dimmed so the user can see the whole queue.
func (job *deletionJob) dimmedPaths() map[string]struct{} {
	job.mut.Lock()
	defer job.mut.Unlock()

	paths := make(map[string]struct{}, len(job.pending)+1)
	if job.currentPath != "" {
		paths[job.currentPath] = struct{}{}
	}
	for _, item := range job.pending {
		paths[item.target.item.GetPath()] = struct{}{}
	}
	return paths
}

// unqueue removes the targets that are still waiting to be deleted. Targets
// already being deleted are reported separately and are not queued again.
func (job *deletionJob) unqueue(targets []deleteTarget) (remaining []deleteTarget, removed, inProgress int) {
	job.mut.Lock()
	defer job.mut.Unlock()

	wanted := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		wanted[target.item.GetPath()] = struct{}{}
	}

	queued := make(map[string]struct{}, len(job.pending))
	kept := job.pending[:0]
	for _, item := range job.pending {
		path := item.target.item.GetPath()
		if _, ok := wanted[path]; ok {
			queued[path] = struct{}{}
			removed++
			continue
		}
		kept = append(kept, item)
	}
	job.pending = kept

	for _, target := range targets {
		path := target.item.GetPath()
		if _, ok := queued[path]; ok {
			continue
		}
		if path == job.currentPath {
			inProgress++
			continue
		}
		remaining = append(remaining, target)
	}
	return remaining, removed, inProgress
}

// finishIfIdle closes the dialog once the queue stayed empty. If an item was
// added in the meantime, the running worker will call this again later.
func (job *deletionJob) finishIfIdle() {
	job.mut.Lock()
	if job.running || len(job.pending) > 0 {
		job.mut.Unlock()
		return
	}
	job.closed = true
	hidden := job.progress.hidden
	job.mut.Unlock()

	ui := job.ui
	ui.finishDeletion(job.progress)
	ui.markedItems = make(map[fs.Item]struct{})
	ui.refreshListing()
	if hidden {
		// Keep the popup on screen for a moment: a hidden deletion can finish
		// before the event loop had a chance to draw it.
		time.AfterFunc(deletionStatusLinger, func() {
			ui.app.QueueUpdateDraw(ui.updateDeletionStatus)
		})
		return
	}
	ui.updateDeletionStatus()
}

// fail stops the job and drops whatever was queued behind the failed item.
func (job *deletionJob) fail() {
	job.mut.Lock()
	job.closed = true
	job.pending = nil
	job.running = false
	job.currentPath = ""
	job.mut.Unlock()

	job.ui.finishDeletion(job.progress)
	// the dropped items are no longer queued, so clear their dimmed colour
	job.ui.refreshListing()
	job.ui.updateDeletionStatus()
}

// finishDeletion removes the progress modal of a finished job and forgets it,
// so that a later deletion can start.
func (ui *UI) finishDeletion(progress *deletionProgress) {
	ui.pages.RemovePage(progress.page)
	ui.deletionMut.Lock()
	if ui.activeDeletion != nil && ui.activeDeletion.progress == progress {
		ui.activeDeletion = nil
	}
	ui.deletionMut.Unlock()
}

// toggleDeletionProgress hides the running foreground deletion dialog so the
// tree can be browsed, or shows it again. The deletion itself keeps running.
func (ui *UI) toggleDeletionProgress() {
	job := ui.activeDeletionJob()
	if job == nil {
		return
	}

	job.mut.Lock()
	hidden := job.progress.hidden
	job.progress.hidden = !hidden
	job.mut.Unlock()

	if hidden {
		ui.pages.AddPage(job.progress.page, job.progress.modal, true, true)
		ui.app.SetFocus(job.progress.modal)
	} else {
		ui.pages.RemovePage(job.progress.page)
		ui.refreshListing()
		ui.app.SetFocus(ui.table)
	}
	ui.updateDeletionStatus()
}

// refreshListing redraws the current directory without moving the cursor, so
// queued items can change colour without interrupting navigation.
func (ui *UI) refreshListing() {
	if ui.currentDir == nil {
		return
	}

	row, _ := ui.table.GetSelection()
	x, y := ui.table.GetOffset()
	ui.showDir()
	ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
	ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
}
