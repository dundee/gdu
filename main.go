package main

import (
	"io/ioutil"
	"os"
)

func main() {
	var topDir string
	if len(os.Args) > 1 {
		topDir = os.Args[1]
	} else {
		topDir = "."
	}

	ui := CreateUI(topDir)

	// go func() {
	// 	ui.QueueUpdate(func() {
	// 		table.SetCell(2, 0, tview.NewTableCell("cc"))
	// 	})
	// }()

	ui.ShowDir(topDir)
	ui.StartUILoop()
}

func processDir(path string) Dir {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return Dir{}
	}

	dir := Dir{
		name: path,
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
