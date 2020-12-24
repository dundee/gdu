package main

import (
	"io/ioutil"
	"os"
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

	subDirsChan := make(chan *File, len(files))

	dir := File{
		name:      filepath.Base(path),
		path:      path,
		isDir:     true,
		itemCount: 1,
		files:     make([]*File, len(files)),
	}
	dirCount := 0

	index := 0
	for _, f := range files {
		if f.IsDir() {
			dirCount++
			go func(subDirsChan chan *File, f os.FileInfo) {
				file = processDir(filepath.Join(path, f.Name()), statusChannel)
				file.parent = &dir
				subDirsChan <- file
			}(subDirsChan, f)
		} else {
			file = &File{
				name:      f.Name(),
				path:      filepath.Join(path, f.Name()),
				size:      f.Size(),
				itemCount: 1,
				parent:    &dir,
			}

			dir.size += file.size
			dir.itemCount += file.itemCount
			dir.files[index] = file
			index++
		}
	}

	filesCount := len(files) - dirCount
	for i := 0; i < dirCount; i++ {
		file = <-subDirsChan
		dir.size += file.size
		dir.itemCount += file.itemCount
		dir.files[filesCount+i] = file

		select {
		case statusChannel <- CurrentProgress{
			currentItemName: file.path,
			itemCount:       file.itemCount,
			totalSize:       file.size,
		}:
		default:
		}
	}

	sort.Slice(dir.files, func(i, j int) bool {
		return dir.files[i].size > dir.files[j].size
	})

	return &dir
}
