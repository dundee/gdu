package fs

import (
	"io"
	"iter"
	"time"

	"github.com/maruel/natural"
)

// SortBy represents the field to sort files by
type SortBy int

const (
	SortBySize SortBy = iota
	SortByName
	SortByItemCount
	SortByMtime
	SortByApparentSize
)

// SortOrder represents the sort direction
type SortOrder int

const (
	SortAsc SortOrder = iota
	SortDesc
)

// Item is a FS item (file or dir)
type Item interface {
	GetPath() string
	GetName() string
	GetFlag() rune
	IsDir() bool
	GetSize() int64
	GetType() string
	GetUsage() int64
	GetMtime() time.Time
	GetItemCount() int64
	GetParent() Item
	SetParent(Item)
	GetMultiLinkedInode() uint64
	EncodeJSON(writer io.Writer, topLevel bool, attributes JSONAttributes) error
	GetItemStats(linkedItems HardLinkedItems, filteringFiles bool) (itemCount int64, size, usage int64)
	UpdateStats(linkedItems HardLinkedItems)
	UpdateStatsWithFileFiltering(linkedItems HardLinkedItems)
	AddFile(Item)
	GetFiles(SortBy, SortOrder) iter.Seq[Item]
	GetFilesLocked(SortBy, SortOrder) iter.Seq[Item]
	RemoveFile(Item)
	RemoveFileByName(name string)
	RLock() func()
}

// SymlinkItem is an optional interface implemented by items that carry a
// symlink target. Consumers type-assert an Item to it to display the target
// (e.g. "name -> target"); items that are not symlinks return an empty string.
type SymlinkItem interface {
	GetSymlinkTarget() string
}

// Files - slice of pointers to File
type Files []Item

// HardLinkedItems maps inode number to array of all hard linked items
type HardLinkedItems map[uint64]Files

// IndexOf searches File in Files and returns its index
func (f Files) IndexOf(file Item) (int, bool) {
	for i, item := range f {
		if item == file {
			return i, true
		}
	}
	return 0, false
}

// FindByName searches name in Files and returns its index
func (f Files) FindByName(name string) (int, bool) {
	for i, item := range f {
		if item.GetName() == name {
			return i, true
		}
	}
	return 0, false
}

// Remove removes File from Files
func (f Files) Remove(file Item) Files {
	index, ok := f.IndexOf(file)
	if !ok {
		return f
	}
	return append(f[:index], f[index+1:]...)
}

// RemoveByName removes File from Files
func (f Files) RemoveByName(name string) Files {
	index, ok := f.FindByName(name)
	if !ok {
		return f
	}
	return append(f[:index], f[index+1:]...)
}

func (f Files) Len() int      { return len(f) }
func (f Files) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f Files) Less(i, j int) bool {
	if f[i].GetUsage() != f[j].GetUsage() {
		return f[i].GetUsage() < f[j].GetUsage()
	}
	// if usage is the same, sort by name
	return natural.Less(f[i].GetName(), f[j].GetName())
}

// ByApparentSize sorts files by apparent size
type ByApparentSize Files

func (f ByApparentSize) Len() int      { return len(f) }
func (f ByApparentSize) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ByApparentSize) Less(i, j int) bool {
	if f[i].GetSize() != f[j].GetSize() {
		return f[i].GetSize() < f[j].GetSize()
	}
	// if size is the same, sort by name
	return natural.Less(f[i].GetName(), f[j].GetName())
}

// DisplayedItemCount returns the item count as it is presented to the user.
//
// For a directory it is the number of items contained in its tree, excluding
// the directory itself (GetItemCount counts the directory too). For a file it
// is 1, i.e. the file counts itself. The asymmetry is deliberate: a row for a
// file reports the one file it stands for, while a row for a directory reports
// what is inside it.
//
// This is the authoritative value: it is what every frontend renders and what
// SortByItemCount orders by, so the number on screen and the ordering always
// agree.
func DisplayedItemCount(item Item) int64 {
	if item.IsDir() {
		return item.GetItemCount() - 1
	}
	return item.GetItemCount()
}

// ByItemCount sorts files by item count
type ByItemCount Files

func (f ByItemCount) Len() int      { return len(f) }
func (f ByItemCount) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ByItemCount) Less(i, j int) bool {
	ci, cj := DisplayedItemCount(f[i]), DisplayedItemCount(f[j])
	if ci != cj {
		return ci < cj
	}
	// if item count is the same, sort by name
	return natural.Less(f[i].GetName(), f[j].GetName())
}

// ByName sorts files by name
type ByName Files

func (f ByName) Len() int           { return len(f) }
func (f ByName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByName) Less(i, j int) bool { return natural.Less(f[i].GetName(), f[j].GetName()) }

// ByMtime sorts files by name
type ByMtime Files

func (f ByMtime) Len() int      { return len(f) }
func (f ByMtime) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ByMtime) Less(i, j int) bool {
	if !f[i].GetMtime().Equal(f[j].GetMtime()) {
		return f[i].GetMtime().Before(f[j].GetMtime())
	}
	// if item count is the same, sort by name
	return natural.Less(f[i].GetName(), f[j].GetName())
}

// ParseSortBy converts a string to SortBy
func ParseSortBy(s string) SortBy {
	switch s {
	case "name":
		return SortByName
	case "size":
		return SortBySize
	case "itemCount":
		return SortByItemCount
	case "mtime":
		return SortByMtime
	default:
		return SortBySize
	}
}

// ParseSortOrder converts a string to SortOrder
func ParseSortOrder(s string) SortOrder {
	if s == "asc" {
		return SortAsc
	}
	return SortDesc
}
