package run

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/dundee/gdu/build"
	"github.com/dundee/gdu/common"
	"github.com/dundee/gdu/device"
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
	NoCross          bool
}

// Run starts gdu main logic
func Run(flags *RunFlags, args []string, istty bool, writer io.Writer, app common.Application, getter device.DevicesInfoGetter) error {
	if flags.ShowVersion {
		fmt.Fprintln(writer, "Version:\t", build.Version)
		fmt.Fprintln(writer, "Built time:\t", build.Time)
		fmt.Fprintln(writer, "Built user:\t", build.User)
		return nil
	}

	var path string
	var ui common.UI

	f, err := os.OpenFile(flags.LogFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Error opening log file: %w", err)
	}
	defer f.Close()
	log.SetOutput(f)

	if len(args) == 1 {
		path = args[0]
	} else {
		path = "."
	}

	if flags.NonInteractive || !istty {
		ui = stdout.CreateStdoutUI(
			writer,
			!flags.NoColor && istty,
			!flags.NoProgress && istty,
			flags.ShowApparentSize,
		)
	} else {
		ui = tui.CreateUI(app, !flags.NoColor, flags.ShowApparentSize)

		if !flags.NoColor {
			tview.Styles.TitleColor = tcell.NewRGBColor(27, 161, 227)
		}
	}

	if flags.NoCross {
		mounts, err := getter.GetMounts()
		if err != nil {
			return fmt.Errorf("Error loading mount points: %w", err)
		}
		paths := device.GetNestedMountpointsPaths(path, mounts)
		flags.IgnoreDirs = append(flags.IgnoreDirs, paths...)
	}

	ui.SetIgnoreDirPaths(flags.IgnoreDirs)

	if flags.ShowDisks {
		if err := ui.ListDevices(getter); err != nil {
			return fmt.Errorf("Error loading mount points: %w", err)
		}
	} else {
		ui.AnalyzePath(path, nil)
	}

	return ui.StartUILoop()
}
