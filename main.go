package main

import (
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
