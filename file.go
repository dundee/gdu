package main

import (
	"os"
)

// File struct
type File struct {
	name      string
	path      string
	size      int64
	itemCount int
	isDir     bool
	files     Files
	parent    *File
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

// RemoveFile removes file from dir
func (f *File) RemoveFile(file *File) {
	error := os.RemoveAll(file.path)
	if error != nil {
		panic(error)
	}

	f.files = f.files.Remove(file)

	cur := f
	for {
		cur.itemCount -= file.itemCount
		cur.size -= file.size

		if cur.parent == nil {
			break
		}
		cur = cur.parent
	}
}

// UpdateStats recursively updates size and item count
func (f *File) UpdateStats() {
	if !f.isDir {
		return
	}
	var totalSize int64
	var itemCount int
	for _, entry := range f.files {
		entry.UpdateStats()
		totalSize += entry.size
		itemCount += entry.itemCount
	}
	f.itemCount = itemCount + 1
	f.size = totalSize
}
