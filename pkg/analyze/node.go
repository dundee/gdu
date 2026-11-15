package analyze

import (
	"errors"
	"fmt"
	"io"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// Node is the top of a tree an analyzer built, or any item of such a tree:
// an fs.Item whose totals can be computed and which can be exported.
type Node interface {
	fs.Item
	// UpdateStats computes the totals of the tree topped by the item and
	// returns the index of its hard-linked files. filteringFiles tells the
	// totals that the scan left out files by their name or mtime.
	UpdateStats(filteringFiles bool) *fs.HardLinks
	// EncodeJSON writes the tree topped by the item in the format of ncdu's
	// export; topLevel tells whether the item tops the export.
	EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error
}

// treeItem is what the items of the trees of this package implement beyond
// fs.Item.
type treeItem interface {
	Node
	// itemStats computes the totals of the item, counting the hard-linked
	// files of its tree in links, and returns them
	itemStats(links *fs.HardLinks, filteringFiles bool) (itemCount, size, usage int64)
	SetParent(parent fs.Item)
}

// changeableDir is a directory whose entries can change.
type changeableDir interface {
	fs.Item
	// AddFile adds item to the entries, leaving the totals as they are
	AddFile(item fs.Item)
	// RemoveFile removes item from the entries and takes it out of the
	// totals of the directory and its ancestors
	RemoveFile(item fs.Item)
}

// ErrNotChangeable is returned when changing an item of a tree that cannot
// change, such as the entries of a browsed archive.
var ErrNotChangeable = errors.New("the tree cannot be changed here")

// RemoveEntry removes item, an entry of dir, from the tree, and takes it out
// of the totals of dir and its ancestors, as when the item was deleted.
func RemoveEntry(dir, item fs.Item) error {
	d, ok := dir.(changeableDir)
	if !ok {
		return fmt.Errorf("removing %s: %w", item.GetPath(), ErrNotChangeable)
	}
	d.RemoveFile(item)
	return nil
}

// ReplaceEntry puts entry in the place of old, an entry of dir, and counts
// it in the totals of dir and its ancestors instead of old.
func ReplaceEntry(dir, old, entry fs.Item) error {
	d, ok := dir.(changeableDir)
	if !ok {
		return fmt.Errorf("replacing %s: %w", old.GetPath(), ErrNotChangeable)
	}
	child, ok := entry.(treeItem)
	if !ok {
		return fmt.Errorf("replacing %s: %w", old.GetPath(), ErrNotChangeable)
	}
	child.SetParent(dir)
	d.RemoveFile(old)
	d.AddFile(entry)
	if holder, ok := dir.(statsHolder); ok {
		adjustChain(holder, entry.GetItemCount(), entry.GetSize(), entry.GetUsage())
	}
	return nil
}

// totalsOf returns the totals of item, an entry of a tree being totalled.
// A Node of another package computes its totals on its own, and any other
// item keeps the totals it reports.
func totalsOf(item fs.Item, links *fs.HardLinks, filteringFiles bool) (itemCount, size, usage int64) {
	switch t := item.(type) {
	case treeItem:
		return t.itemStats(links, filteringFiles)
	case Node:
		t.UpdateStats(filteringFiles)
	}
	return item.GetItemCount(), item.GetSize(), item.GetUsage()
}

// setParent makes parent the directory holding item.
func setParent(item, parent fs.Item) {
	if t, ok := item.(treeItem); ok {
		t.SetParent(parent)
	}
}

var (
	_ treeItem      = (*File)(nil)
	_ treeItem      = (*Dir)(nil)
	_ treeItem      = (*StoredDir)(nil)
	_ treeItem      = (*SqliteItem)(nil)
	_ treeItem      = (*ZipDir)(nil)
	_ treeItem      = (*ZipFile)(nil)
	_ treeItem      = (*TarDir)(nil)
	_ treeItem      = (*TarFile)(nil)
	_ changeableDir = (*Dir)(nil)
	_ changeableDir = (*StoredDir)(nil)
	_ changeableDir = (*SqliteItem)(nil)
)
