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

	// ui.currentDir = processDir(topDir)
	// ui.ShowDir()

	go func() {
		ui.app.QueueUpdateDraw(func() {
			ui.currentDir = processDir(topDir)
			ui.ShowDir()
			ui.pages.HidePage("modal")
		})
	}()

	ui.StartUILoop()
}
