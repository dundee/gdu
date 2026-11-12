package tui

import (
	"path/filepath"

	"github.com/rivo/tview"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/dundee/gdu/v5/pkg/analyze"
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
	return ui.newDeleter(analyze.ScannedRootPaths(ui.topDir))
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

	acting := action.Acting()
	title := cases.Title(language.English).String(acting)
	modal := tview.NewModal().SetText(title + " " + tview.Escape(targets[0].item.GetName()) + "...")
	ui.pages.AddPage(acting, modal, true, true)

	currentRow, _ := ui.table.GetSelection()

	go func() {
		for i, target := range targets {
			name := tview.Escape(target.item.GetName())
			if i > 0 {
				ui.app.QueueUpdateDraw(func() {
					modal.SetText(title + " " + name + "...")
				})
			}

			if err := deleter.Delete(target.dir, target.item, action.mode()); err != nil {
				msg := "Can't " + action.Verb() + " " + name
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

		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage(acting)
			ui.markedItems = make(map[fs.Item]struct{})
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
