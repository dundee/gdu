package main

import "io/ioutil"

// Dir struct
type Dir struct {
	name  string
	files []File
}

// File struct
type File struct {
	name  string
	size  int64
	isDir bool
	dir   *Dir
}

func processDir(path string) Dir {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return Dir{}
	}

	dir := Dir{
		name:  path,
		files: make([]File, len(files)),
	}

	for i, f := range files {
		file := File{
			name: f.Name(),
			size: f.Size(),
		}
		dir.files[i] = file
	}

	return dir
}
