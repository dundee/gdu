package analyze

import (
	"os"
	"path/filepath"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// File struct
type File struct {
	Mtime  time.Time
	Parent fs.Item
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

// GetParent returns parent dir
func (f *File) GetParent() fs.Item {
	return f.Parent
}

// SetParent sets parent dir
func (f *File) SetParent(parent fs.Item) {
	f.Parent = parent
}

// GetPath returns absolute Get of the file
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

func (f *File) alreadyCounted(linkedItems fs.HardLinkedItems) bool {
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

// GetItemStats returns 1 as count of items, apparent usage and real usage of this file
func (f *File) GetItemStats(linkedItems fs.HardLinkedItems) (int, int64, int64) {
	if f.alreadyCounted(linkedItems) {
		return 1, 0, 0
	}
	return 1, f.GetSize(), f.GetUsage()
}

// UpdateStats does nothing on file
func (f *File) UpdateStats(linkedItems fs.HardLinkedItems) {}

// GetFiles returns all files in directory
func (f *File) GetFiles() fs.Files {
	return fs.Files{}
}

// SetFiles panics on file
func (f *File) SetFiles(files fs.Files) {
	panic("SetFiles should not be called on file")
}

// AddFile panics on file
func (f *File) AddFile(item fs.Item) {
	panic("AddFile should not be called on file")
}

// Dir struct
type Dir struct {
	*File
	BasePath  string
	Files     fs.Files
	ItemCount int
}

// AddFile add item fo files
func (f *Dir) AddFile(item fs.Item) {
	f.Files = append(f.Files, item)
}

// GetFiles returns all files in directory
func (f *Dir) GetFiles() fs.Files {
	return f.Files
}

// SetFiles sets files in directory
func (f *Dir) SetFiles(files fs.Files) {
	f.Files = files
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

// GetPath returns absolute path of the file
func (f *Dir) GetPath() string {
	if f.BasePath != "" {
		return filepath.Join(f.BasePath, f.Name)
	}
	if f.Parent != nil {
		return filepath.Join(f.Parent.GetPath(), f.Name)
	}
	return f.Name
}

// GetItemStats returns item count, apparent usage and real usage of this dir
func (f *Dir) GetItemStats(linkedItems fs.HardLinkedItems) (int, int64, int64) {
	f.UpdateStats(linkedItems)
	return f.ItemCount, f.GetSize(), f.GetUsage()
}

// UpdateStats recursively updates size and item count
func (f *Dir) UpdateStats(linkedItems fs.HardLinkedItems) {
	totalSize := int64(4096)
	totalUsage := int64(4096)
	var itemCount int
	for _, entry := range f.Files {
		count, size, usage := entry.GetItemStats(linkedItems)
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

// RemoveItemFromDir removes item from dir
func RemoveItemFromDir(dir fs.Item, item fs.Item) error {
	err := os.RemoveAll(item.GetPath())
	if err != nil {
		return err
	}

	dir.SetFiles(dir.GetFiles().Remove(item))

	cur := dir.(*Dir)
	for {
		cur.ItemCount -= item.GetItemCount()
		cur.Size -= item.GetSize()
		cur.Usage -= item.GetUsage()

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent.(*Dir)
	}
	return nil
}

// EmptyFileFromDir empty file from dir
func EmptyFileFromDir(dir fs.Item, file fs.Item) error {
	err := os.Truncate(file.GetPath(), 0)
	if err != nil {
		return err
	}

	cur := dir.(*Dir)
	for {
		cur.Size -= file.GetSize()
		cur.Usage -= file.GetUsage()

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent.(*Dir)
	}

	dir.SetFiles(dir.GetFiles().Remove(file))
	newFile := &File{
		Name:   file.GetName(),
		Flag:   file.GetFlag(),
		Size:   0,
		Parent: dir,
	}
	dir.AddFile(newFile)

	return nil
}
