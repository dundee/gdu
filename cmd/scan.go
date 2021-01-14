package cmd

import (
	"flag"
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

	f, err := os.OpenFile(flags.logFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	if flags.nonInteractive || !istty {
		if len(args) == 1 {
			ui := stdout.CreateStdoutUI(writer, !flags.noColor && istty, !flags.noProgress && istty)
			ui.SetIgnoreDirPaths(flags.ignoreDirs)
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

	if !flags.noColor {
		tview.Styles.TitleColor = tcell.NewRGBColor(27, 161, 227)
	}

	ui := tui.CreateUI(screen, !flags.noColor)
	ui.SetIgnoreDirPaths(flags.ignoreDirs)

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
