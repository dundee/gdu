package analyze

import (
	"io"
	"os"
	"path/filepath"
	"time"
)

// HardLinkedItems maps inode number to array of all hard linked items
type HardLinkedItems map[uint64]Files

// Item is fs item (file or dir)
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
	GetParent() *Dir
	GetMultiLinkedInode() uint64
	EncodeJSON(writer io.Writer, topLevel bool) error
	getItemStats(linkedItems HardLinkedItems) (int, int64, int64)
}

// File struct
type File struct {
	Mtime  time.Time
	Parent *Dir
	Name   string
	Size   int64
	Usage  int64
	Mli    uint64
	Flag   rune
}

// GetName returns name of dir
func (f *File) GetName() string {
	return f.Name
}

// IsDir returns false for file
func (f *File) IsDir() bool {
	return false
}

// GetParent retruns parent dir
func (f *File) GetParent() *Dir {
	return f.Parent
}

// GetPath retruns absolute Get of the file
func (f *File) GetPath() string {
	return filepath.Join(f.Parent.GetPath(), f.Name)
}

// GetFlag returns flag of the file
func (f *File) GetFlag() rune {
	return f.Flag
}

// GetSize returns size of the file
func (f *File) GetSize() int64 {
	return f.Size
}

// GetUsage returns usage of the file
func (f *File) GetUsage() int64 {
	return f.Usage
}

// GetMtime returns mtime of the file
func (f *File) GetMtime() time.Time {
	return f.Mtime
}

// GetType returns name type of item
func (f *File) GetType() string {
	switch f.Flag {
	case '@':
		return "Other"
	}
	return "File"
}

// GetItemCount returns 1 for file
func (f *File) GetItemCount() int {
	return 1
}

// GetMultiLinkedInode returns inode number of multilinked file
func (f *File) GetMultiLinkedInode() uint64 {
	return f.Mli
}

func (f *File) alreadyCounted(linkedItems HardLinkedItems) bool {
	mli := f.Mli
	counted := false
	if mli > 0 {
		if _, ok := linkedItems[mli]; ok {
			f.Flag = 'H'
			counted = true
		}
		linkedItems[mli] = append(linkedItems[mli], f)
	}
	return counted
}

func (f *File) getItemStats(linkedItems HardLinkedItems) (int, int64, int64) {
	if f.alreadyCounted(linkedItems) {
		return 1, 0, 0
	}
	return 1, f.GetSize(), f.GetUsage()
}

// Dir struct
type Dir struct {
	*File
	BasePath  string
	Files     Files
	ItemCount int
}

// GetType returns name type of item
func (f *Dir) GetType() string {
	return "Directory"
}

// GetItemCount returns number of files in dir
func (f *Dir) GetItemCount() int {
	return f.ItemCount
}

// IsDir returns true for dir
func (f *Dir) IsDir() bool {
	return true
}

// GetPath retruns absolute path of the file
func (f *Dir) GetPath() string {
	if f.BasePath != "" {
		return filepath.Join(f.BasePath, f.Name)
	}
	return filepath.Join(f.Parent.GetPath(), f.Name)
}

func (f *Dir) getItemStats(linkedItems HardLinkedItems) (int, int64, int64) {
	f.UpdateStats(linkedItems)
	return f.ItemCount, f.GetSize(), f.GetUsage()
}

// UpdateStats recursively updates size and item count
func (f *Dir) UpdateStats(linkedItems HardLinkedItems) {
	totalSize := int64(4096)
	totalUsage := int64(4096)
	var itemCount int
	for _, entry := range f.Files {
		count, size, usage := entry.getItemStats(linkedItems)
		totalSize += size
		totalUsage += usage
		itemCount += count

		if entry.GetMtime().After(f.Mtime) {
			f.Mtime = entry.GetMtime()
		}

		switch entry.GetFlag() {
		case '!', '.':
			if f.Flag != '!' {
				f.Flag = '.'
			}
		}
	}
	f.ItemCount = itemCount + 1
	f.Size = totalSize
	f.Usage = totalUsage
}

// Files - slice of pointers to File
type Files []Item

// Append addes one item to Files
func (f *Files) Append(file Item) {
	slice := *f
	slice = append(slice, file)
	*f = slice
}

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

func (f Files) Len() int           { return len(f) }
func (f Files) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f Files) Less(i, j int) bool { return f[i].GetUsage() > f[j].GetUsage() }

// ByApparentSize sorts files by apparent size
type ByApparentSize Files

func (f ByApparentSize) Len() int           { return len(f) }
func (f ByApparentSize) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByApparentSize) Less(i, j int) bool { return f[i].GetSize() > f[j].GetSize() }

// ByItemCount sorts files by item count
type ByItemCount Files

func (f ByItemCount) Len() int           { return len(f) }
func (f ByItemCount) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByItemCount) Less(i, j int) bool { return f[i].GetItemCount() > f[j].GetItemCount() }

// ByName sorts files by name
type ByName Files

func (f ByName) Len() int           { return len(f) }
func (f ByName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByName) Less(i, j int) bool { return f[i].GetName() > f[j].GetName() }

// ByMtime sorts files by name
type ByMtime Files

func (f ByMtime) Len() int           { return len(f) }
func (f ByMtime) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByMtime) Less(i, j int) bool { return f[i].GetMtime().After(f[j].GetMtime()) }

// RemoveItemFromDir removes item from dir
func RemoveItemFromDir(dir *Dir, item Item) error {
	err := os.RemoveAll(item.GetPath())
	if err != nil {
		return err
	}

	dir.Files = dir.Files.Remove(item)

	cur := dir
	for {
		cur.ItemCount -= item.GetItemCount()
		cur.Size -= item.GetSize()
		cur.Usage -= item.GetUsage()

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent
	}
	return nil
}

// EmptyFileFromDir empty file from dir
func EmptyFileFromDir(dir *Dir, file Item) error {
	err := os.Truncate(file.GetPath(), 0)
	if err != nil {
		return err
	}

	cur := dir
	for {
		cur.Size -= file.GetSize()
		cur.Usage -= file.GetUsage()

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent
	}

	dir.Files = dir.Files.Remove(file)
	newFile := &File{
		Name:   file.GetName(),
		Flag:   file.GetFlag(),
		Size:   0,
		Parent: dir,
	}
	dir.Files.Append(newFile)

	return nil
}
