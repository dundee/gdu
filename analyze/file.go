package analyze

import (
	"os"
	"path/filepath"
)

// File struct
type File struct {
	Name           string
	BasePath       string
	Flag           rune
	Size           int64
	Usage          int64
	ItemCount      int
	IsDir          bool
	Files          Files
	Parent         *File
	MutliLinkInode uint64 // Inode number of file with multiple links (hard link)
}

// AlreadyCountedHardlinks holds all files with hardlinks that have already been counted
type AlreadyCountedHardlinks map[uint64]bool

// Path retruns absolute path of the file
func (f *File) Path() string {
	if f.BasePath != "" {
		return filepath.Join(f.BasePath, f.Name)
	}
	return filepath.Join(f.Parent.Path(), f.Name)
}

// RemoveFileFromDir removes file from dir
func RemoveFileFromDir(dir *File, file *File) error {
	error := os.RemoveAll(file.Path())
	if error != nil {
		return error
	}

	dir.Files = dir.Files.Remove(file)

	cur := dir
	for {
		cur.ItemCount -= file.ItemCount
		cur.Size -= file.Size
		cur.Usage -= file.Usage

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent
	}
	return nil
}

// UpdateStats recursively updates size and item count
func (f *File) UpdateStats(links AlreadyCountedHardlinks) {
	if !f.IsDir {
		return
	}
	totalSize := int64(4096)
	totalUsage := int64(4096)
	var itemCount int
	for _, entry := range f.Files {
		if entry.IsDir {
			entry.UpdateStats(links)
		}

		switch entry.Flag {
		case '!', '.':
			if f.Flag != '!' {
				f.Flag = '.'
			}
		}

		itemCount += entry.ItemCount

		if entry.MutliLinkInode > 0 {
			if !links[entry.MutliLinkInode] {
				links[entry.MutliLinkInode] = true
			} else {
				entry.Flag = 'H'
				continue
			}
		}
		totalSize += entry.Size
		totalUsage += entry.Usage
	}
	f.ItemCount = itemCount + 1
	f.Size = totalSize
	f.Usage = totalUsage
}

// Files - slice of pointers to File
type Files []*File

// IndexOf searches File in Files and returns its index
func (f Files) IndexOf(file *File) (int, bool) {
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
		if item.Name == name {
			return i, true
		}
	}
	return 0, false
}

// Remove removes File from Files
func (f Files) Remove(file *File) Files {
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
func (f Files) Less(i, j int) bool { return f[i].Usage > f[j].Usage }

// ByApparentSize sorts files by apparent size
type ByApparentSize Files

func (f ByApparentSize) Len() int           { return len(f) }
func (f ByApparentSize) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByApparentSize) Less(i, j int) bool { return f[i].Size > f[j].Size }

// ByItemCount sorts files by item count
type ByItemCount Files

func (f ByItemCount) Len() int           { return len(f) }
func (f ByItemCount) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByItemCount) Less(i, j int) bool { return f[i].ItemCount > f[j].ItemCount }

// ByName sorts files by name
type ByName Files

func (f ByName) Len() int           { return len(f) }
func (f ByName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByName) Less(i, j int) bool { return f[i].Name > f[j].Name }
