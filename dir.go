package main

import (
	"io/ioutil"
	"path/filepath"
)

// File struct
type File struct {
	name      string
	path      string
	size      int64
	itemCount int
	isDir     bool
	files     []*File
	parent    *File
}

func processDir(path string) *File {
	var file *File
	path, _ = filepath.Abs(path)

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return &File{}
	}

	dir := File{
		name:      filepath.Base(path),
		path:      path,
		isDir:     true,
		itemCount: 0,
		files:     make([]*File, len(files)),
	}

	for i, f := range files {
		if f.IsDir() {
			file = processDir(filepath.Join(path, f.Name()))
			file.parent = &dir
		} else {
			file = &File{
				name:      f.Name(),
				path:      filepath.Join(path, f.Name()),
				size:      f.Size(),
				itemCount: 1,
				parent:    &dir,
			}
		}

		dir.size += file.size
		dir.itemCount += file.itemCount
		dir.files[i] = file
	}

	return &dir
}
