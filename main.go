package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/dundee/gdu/analyze"
	"github.com/dundee/gdu/cli"
	"github.com/dundee/gdu/stdout"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-isatty"
	"github.com/rivo/tview"
)

// AppVersion stores the current version of the app
var AppVersion = "development"

func main() {
	logFile := flag.String("log-file", "/dev/null", "Path to a logfile")
	ignoreDirPaths := flag.String("ignore-dir", "/proc,/dev,/sys,/run", "Absolute paths to ignore (separated by comma)")
	showVersion := flag.Bool("v", false, "Prints version")
	noColor := flag.Bool("no-color", false, "Do not use colorized output")
	nonInteractive := flag.Bool("non-interactive", false, "Do not run in interactive mode")
	noProgress := flag.Bool("no-progress", false, "Do not show progress")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of gdu: [flags] [directory to scan]\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	args := flag.Args()

	istty := isatty.IsTerminal(os.Stdout.Fd())

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

	if *nonInteractive || !istty {
		if len(args) == 1 {
			ui := stdout.CreateStdoutUI(os.Stdout, !*noColor, !*noProgress && istty)
			ui.SetIgnoreDirPaths(strings.Split(*ignoreDirPaths, ","))
			ui.AnalyzePath(args[0], analyze.ProcessDir)
		} else {
			flag.Usage()
		}
		return
	}

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

	if len(args) == 1 {
		ui.AnalyzePath(args[0], analyze.ProcessDir, nil)
	} else {
		if runtime.GOOS == "linux" {
			ui.ListDevices()
		} else {
			ui.AnalyzePath(".", analyze.ProcessDir, nil)
		}
	}

	ui.StartUILoop()
}
