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
	statusChannel := make(chan CurrentProgress)
	go ui.updateProgress(statusChannel)

	go func() {
		ui.currentDir = processDir(topDir, statusChannel)

		ui.app.QueueUpdateDraw(func() {
			ui.ShowDir()
			ui.pages.HidePage("modal")
		})
	}()

	ui.StartUILoop()
}
