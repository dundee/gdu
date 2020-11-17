package main

type Dir struct {
	name string
	files []File
}

type File struct {
	name string
	size int64
	isDir bool
	dir *Dir
}