package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/dundee/gdu/cli"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// AppVersion stores the current version of the app
var AppVersion = "development"

func main() {
	logFile := flag.String("log-file", "/dev/null", "Path to a logfile")
	ignoreDirPaths := flag.String("ignore-dir", "/proc,/dev,/sys,/run", "Absolute paths to ignore (separated by comma)")
	showVersion := flag.Bool("v", false, "Prints version")
	noColor := flag.Bool("no-color", false, "Do not use colorized output")
	flag.Parse()

	if *showVersion {
		fmt.Println("Version:\t", AppVersion)
		return
	}

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

	if !*noColor {
		tview.Styles.TitleColor = tcell.NewRGBColor(27, 161, 227)
	}

	ui := cli.CreateUI(screen, !*noColor)
	ui.SetIgnoreDirPaths(strings.Split(*ignoreDirPaths, ","))

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
