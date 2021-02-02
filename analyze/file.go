package analyze

import (
	"errors"
	"os"
	"path/filepath"
)

// File struct
type File struct {
	Name      string
	BasePath  string
	Size      int64
	Usage     int64
	ItemCount int
	IsDir     bool
	Files     Files
	Parent    *File
}

// ErrNotFound is returned when File item is not found in Files
var ErrNotFound = errors.New("File item not found")

// Path retruns absolute path of the file
func (f *File) Path() string {
	if f.BasePath != "" {
		return filepath.Join(f.BasePath, f.Name)
	}
	return filepath.Join(f.Parent.Path(), f.Name)
}

// RemoveFile removes file from dir
func (f *File) RemoveFile(file *File) error {
	error := os.RemoveAll(file.Path())
	if error != nil {
		return error
	}

	f.Files = f.Files.Remove(file)

	cur := f
	for {
		cur.ItemCount -= file.ItemCount
		cur.Size -= file.Size

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent
	}
	return nil
}

// UpdateStats recursively updates size and item count
func (f *File) UpdateStats() {
	if !f.IsDir {
		return
	}
	totalSize := int64(4096)
	totalUsage := int64(4096)
	var itemCount int
	for _, entry := range f.Files {
		entry.UpdateStats()
		totalSize += entry.Size
		totalUsage += entry.Usage
		itemCount += entry.ItemCount
	}
	f.ItemCount = itemCount + 1
	f.Size = totalSize
	f.Usage = totalUsage
}

// Files - slice of pointers to File
type Files []*File

// IndexOf searches File in Files and returns its index, or -1
func (f Files) IndexOf(file *File) (int, error) {
	for i, item := range f {
		if item == file {
			return i, nil
		}
	}
	return 0, ErrNotFound
}

// FindByName searches name in Files and returns its index, or -1
func (f Files) FindByName(name string) (int, error) {
	for i, item := range f {
		if item.Name == name {
			return i, nil
		}
	}
	return 0, ErrNotFound
}

// Remove removes File from Files
func (f Files) Remove(file *File) Files {
	index, err := f.IndexOf(file)
	if err != nil {
		return f
	}
	return append(f[:index], f[index+1:]...)
}

// RemoveByName removes File from Files
func (f Files) RemoveByName(name string) Files {
	index, err := f.FindByName(name)
	if err != nil {
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
