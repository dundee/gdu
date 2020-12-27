package main

import (
	"os"
	"runtime"

	"github.com/gdamore/tcell/v2"
)

func main() {
	screen, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}
	screen.Init()

	ui := CreateUI(screen)

	if len(os.Args) > 1 {
		ui.AnalyzePath(os.Args[1])
	} else {
		if runtime.GOOS == "linux" {
			ui.ListDevices()
		} else {
			ui.AnalyzePath(".")
		}
	}

	ui.StartUILoop()
}
