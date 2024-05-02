package fs

import (
	"io"
	"time"

	"github.com/maruel/natural"
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
	GetItemCount() int
	GetParent() Item
	SetParent(Item)
	GetMultiLinkedInode() uint64
	EncodeJSON(writer io.Writer, topLevel bool) error
	GetItemStats(linkedItems HardLinkedItems) (itemCount int, size, usage int64)
	UpdateStats(linkedItems HardLinkedItems)
	AddFile(Item)
	GetFiles() Files
	GetFilesLocked() Files
	SetFiles(Files)
	RemoveFile(Item)
	RLock() func()
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

// ByItemCount sorts files by item count
type ByItemCount Files

func (f ByItemCount) Len() int      { return len(f) }
func (f ByItemCount) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ByItemCount) Less(i, j int) bool {
	if f[i].GetItemCount() != f[j].GetItemCount() {
		return f[i].GetItemCount() < f[j].GetItemCount()
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
