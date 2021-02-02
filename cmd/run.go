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

// RunFlags define flags accepted by Run
type RunFlags struct {
	LogFile          string
	IgnoreDirs       []string
	ShowDisks        bool
	ShowApparentSize bool
	ShowVersion      bool
	NoColor          bool
	NonInteractive   bool
	NoProgress       bool
}

// Run starts gdu main logic
func Run(flags *RunFlags, args []string, istty bool, writer io.Writer, testing bool) {
	if flags.ShowVersion {
		fmt.Fprintln(writer, "Version:\t", build.Version)
		fmt.Fprintln(writer, "Built time:\t", build.Time)
		fmt.Fprintln(writer, "Built user:\t", build.User)
		return
	}

	var path string
	var ui tui.CommonUI

	f, err := os.OpenFile(flags.LogFile, os.O_RDWR|os.O_CREATE, 0644)
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

	// we are not able to analyze disk usage on Windows and Plan9
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		flags.ShowApparentSize = true
	}

	if flags.NonInteractive || !istty {
		ui = stdout.CreateStdoutUI(
			writer,
			!flags.NoColor && istty,
			!flags.NoProgress && istty,
			flags.ShowApparentSize,
		)
	} else {
		var screen tcell.Screen

		if testing {
			screen = tcell.NewSimulationScreen("UTF-8")
		} else {
			screen, err = tcell.NewScreen()
			if err != nil {
				panic(err)
			}
		}
		screen.Init()

		ui = tui.CreateUI(screen, !flags.NoColor, flags.ShowApparentSize)

		if !flags.NoColor {
			tview.Styles.TitleColor = tcell.NewRGBColor(27, 161, 227)
		}
	}

	if flags.ShowDisks {
		if runtime.GOOS == "linux" {
			ui.SetIgnoreDirPaths(flags.IgnoreDirs)
			ui.ListDevices(analyze.GetDevicesInfo)
		} else {
			fmt.Fprint(writer, "Listing devices is not yet supported for this platform")
			return
		}
	} else {
		ui.SetIgnoreDirPaths(flags.IgnoreDirs)
		ui.AnalyzePath(path, analyze.ProcessDir, nil)
	}

	if testing {
		return
	}

	switch u := ui.(type) {
	case *tui.UI:
		u.StartUILoop()
	}
}
