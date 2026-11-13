package analyze

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// inodeKey identifies a file with several hard links. Inode numbers are
// unique only within a filesystem, so the device is part of the key. It is
// zero for a file with a single link, and on platforms without inodes.
type inodeKey struct {
	dev, ino uint64
}

// linkTracker records the hard-linked files met by a scan that totals as it
// goes, so that each is counted once however many links lead to it. It is
// safe for concurrent use.
type linkTracker struct {
	seen sync.Map
}

// claim reports whether the file is met for the first time.
func (t *linkTracker) claim(key inodeKey) bool {
	_, seen := t.seen.LoadOrStore(key, struct{}{})
	return !seen
}

// reset forgets every file met, for a new scan.
func (t *linkTracker) reset() {
	t.seen.Clear()
}

// HardLinkKey returns the identity of a file with several hard links, and
// false for any other item.
func HardLinkKey(item fs.Item) (fs.InodeKey, bool) {
	if item.IsDir() {
		return fs.InodeKey{}, false
	}
	ino := item.GetMultiLinkedInode()
	if ino == 0 {
		return fs.InodeKey{}, false
	}
	return fs.InodeKey{Dev: Device(item.GetParent()), Ino: ino}, true
}

// Device returns the device the directory dir lies on, or 0 when it is not
// known.
func Device(dir fs.Item) uint64 {
	if d, ok := dir.(interface{ device() uint64 }); ok {
		return d.device()
	}
	return 0
}

// statsHolder is a directory whose totals change when an entry is removed.
type statsHolder interface {
	fs.Item
	// adjustStats adds the deltas to the totals of the directory alone
	adjustStats(itemCount, size, usage int64)
	// hardLinks returns the index of the hard-linked files the directory
	// keeps as the top of a tree, or nil
	hardLinks() *fs.HardLinks
}

// removeFromTotals takes item, just removed from dir, out of the totals of
// dir and its ancestors.
//
// A hard-linked file stays on the disk while another of its links does: a
// link that was not counted takes no usage with it, and when the counted one
// goes, the next link left in the tree is counted instead.
func removeFromTotals(dir statsHolder, item fs.Item) {
	itemCount, size, usage := item.GetItemCount(), item.GetSize(), item.GetUsage()

	links := treeHardLinks(dir)
	found, removedCounted, nowCounted := links.Remove(item.GetPath(), string(filepath.Separator))
	if !item.IsDir() && found && !removedCounted {
		size, usage = 0, 0
	}
	adjustChain(dir, -itemCount, -size, -usage)

	for _, link := range nowCounted {
		if parent := dirHolding(treeTop(dir), link.GetPath()); parent != nil {
			adjustChain(parent, 0, link.GetSize(), link.GetUsage())
		}
	}
}

// adjustChain adds the deltas to the totals of dir and its ancestors.
func adjustChain(dir statsHolder, itemCount, size, usage int64) {
	for {
		dir.adjustStats(itemCount, size, usage)
		parent, ok := dir.GetParent().(statsHolder)
		if !ok {
			return
		}
		dir = parent
	}
}

func treeTop(item fs.Item) fs.Item {
	for item.GetParent() != nil {
		item = item.GetParent()
	}
	return item
}

// treeHardLinks returns the index kept by the top of the tree holding dir.
func treeHardLinks(dir statsHolder) *fs.HardLinks {
	var links *fs.HardLinks
	for {
		if l := dir.hardLinks(); l != nil {
			links = l
		}
		parent, ok := dir.GetParent().(statsHolder)
		if !ok {
			return links
		}
		dir = parent
	}
}

// dirHolding returns the directory of the tree topped by top holding the
// entry at path. It is found by path, as the index may hold copies of the
// directories of a stored tree rather than the directories themselves.
func dirHolding(top fs.Item, path string) statsHolder {
	dirPath := filepath.Dir(path)
	cur := top
	for cur.GetPath() != dirPath {
		var next fs.Item
		for entry := range cur.GetFilesLocked(fs.SortByName, fs.SortAsc) {
			entryPath := entry.GetPath()
			if entry.IsDir() && (entryPath == dirPath || strings.HasPrefix(dirPath, entryPath+string(filepath.Separator))) {
				next = entry
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	holder, _ := cur.(statsHolder)
	return holder
}
