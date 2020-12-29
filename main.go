package main

import (
	"flag"
	"log"
	"os"
	"runtime"

	"github.com/gdamore/tcell/v2"
)

func main() {
	logFile := flag.String("log-file", "/dev/null", "Path to a logfile")
	flag.Parse()

	f, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	screen, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}
	screen.Init()

	ui := CreateUI(screen)

	args := flag.Args()

	if len(args) == 1 {
		ui.AnalyzePath(args[0])
	} else {
		if runtime.GOOS == "linux" {
			ui.ListDevices()
		} else {
			ui.AnalyzePath(".")
		}
	}

	ui.StartUILoop()
}
