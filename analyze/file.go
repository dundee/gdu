package analyze

import (
	"os"
	"path/filepath"
)

// File struct
type File struct {
	Name      string
	BasePath  string
	Size      int64
	ItemCount int
	IsDir     bool
	Files     Files
	Parent    *File
}

// Path retruns absolute path of the file
func (f *File) Path() string {
	if f.BasePath != "" {
		return filepath.Join(f.BasePath, f.Name)
	}
	return filepath.Join(f.Parent.Path(), f.Name)
}

// RemoveFile removes file from dir
func (f *File) RemoveFile(file *File) {
	error := os.RemoveAll(file.Path())
	if error != nil {
		panic(error)
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
}

// UpdateStats recursively updates size and item count
func (f *File) UpdateStats() {
	if !f.IsDir {
		return
	}
	var totalSize int64
	var itemCount int
	for _, entry := range f.Files {
		entry.UpdateStats()
		totalSize += entry.Size
		itemCount += entry.ItemCount
	}
	f.ItemCount = itemCount + 1
	f.Size = totalSize
}

// Files - slice of pointers to File
type Files []*File

// Find searches File in Files and returns its index, or -1
func (s Files) Find(file *File) int {
	for i, item := range s {
		if item == file {
			return i
		}
	}
	return -1
}

// Remove removes File from Files
func (s Files) Remove(file *File) Files {
	index := s.Find(file)
	if index == -1 {
		return s
	}
	return append(s[:index], s[index+1:]...)
}
