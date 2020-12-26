package main

import (
	"os"

	"github.com/gdamore/tcell/v2"
)

func main() {
	var topDir string
	if len(os.Args) > 1 {
		topDir = os.Args[1]
	} else {
		topDir = "."
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}
	screen.Init()

	ui := CreateUI(topDir, screen)
	statusChannel := make(chan CurrentProgress)
	go ui.updateProgress(statusChannel)

	go func() {
		ui.currentDir = ProcessDir(topDir, statusChannel)

		ui.app.QueueUpdateDraw(func() {
			ui.ShowDir()
			ui.pages.HidePage("progress")
		})
	}()

	ui.StartUILoop()
}
