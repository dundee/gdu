package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/dundee/gdu/analyze"
	"github.com/dundee/gdu/build"
	"github.com/dundee/gdu/stdout"
	"github.com/dundee/gdu/tui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type scanFlags struct {
	logFile        string
	ignoreDirs     []string
	showDisks      bool
	showVersion    bool
	noColor        bool
	nonInteractive bool
	noProgress     bool
}

func scan(flags *scanFlags, args []string, istty bool, writer io.Writer) {
	if flags.showVersion {
		fmt.Fprintln(writer, "Version:\t", build.Version)
		fmt.Fprintln(writer, "Built time:\t", build.Time)
		fmt.Fprintln(writer, "Built user:\t", build.User)
		return
	}

	var path string

	f, err := os.OpenFile(flags.logFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	if len(args) == 1 {
		path = args[0]
	} else {
		path = "."
	}

	if flags.nonInteractive || !istty {
		ui := stdout.CreateStdoutUI(writer, !flags.noColor && istty, !flags.noProgress && istty)
		ui.SetIgnoreDirPaths(flags.ignoreDirs)
		ui.AnalyzePath(path, analyze.ProcessDir)
		return
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}
	screen.Init()

	if !flags.noColor {
		tview.Styles.TitleColor = tcell.NewRGBColor(27, 161, 227)
	}

	ui := tui.CreateUI(screen, !flags.noColor)

	if flags.showDisks {
		if runtime.GOOS == "linux" {
			ui.ListDevices()
		} else {
			fmt.Fprint(writer, "Listing devices is not yet supported for this platform")
			return
		}
	} else {
		ui.SetIgnoreDirPaths(flags.ignoreDirs)
		ui.AnalyzePath(path, analyze.ProcessDir, nil)
	}

	ui.StartUILoop()
}
