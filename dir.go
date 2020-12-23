package main

import (
	"io/ioutil"
	"path/filepath"
	"sort"
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

// CurrentProgress struct
type CurrentProgress struct {
	currentItemName string
	itemCount       int
	totalSize       int64
	done            bool
}

func processDir(path string, statusChannel chan CurrentProgress) *File {
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
			file = processDir(filepath.Join(path, f.Name()), statusChannel)
			file.parent = &dir

			select {
			case statusChannel <- CurrentProgress{
				currentItemName: file.path,
				itemCount:       file.itemCount,
				totalSize:       file.size,
			}:
			default:
			}
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

	sort.Slice(dir.files, func(i, j int) bool {
		return dir.files[i].size > dir.files[j].size
	})

	return &dir
}
