package app

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/dundee/gdu/v4/build"
	"github.com/dundee/gdu/v4/internal/common"
	"github.com/dundee/gdu/v4/pkg/analyze"
	"github.com/dundee/gdu/v4/pkg/device"
	"github.com/dundee/gdu/v4/stdout"
	"github.com/dundee/gdu/v4/tui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI is common interface for both terminal UI and text output
type UI interface {
	ListDevices(getter device.DevicesInfoGetter) error
	AnalyzePath(path string, parentDir *analyze.Dir) error
	SetIgnoreDirPaths(paths []string)
	SetIgnoreDirPatterns(paths []string) error
	SetIgnoreHidden(value bool)
	StartUILoop() error
}

// Flags define flags accepted by Run
type Flags struct {
	LogFile           string
	IgnoreDirs        []string
	IgnoreDirPatterns []string
	MaxCores          int
	ShowDisks         bool
	ShowApparentSize  bool
	ShowVersion       bool
	NoColor           bool
	NonInteractive    bool
	NoProgress        bool
	NoCross           bool
	NoHidden          bool
}

// App defines the main application
type App struct {
	Args    []string
	Flags   *Flags
	Istty   bool
	Writer  io.Writer
	TermApp common.TermApplication
	Getter  device.DevicesInfoGetter
}

// Run starts gdu main logic
func (a *App) Run() error {
	if a.Flags.ShowVersion {
		fmt.Fprintln(a.Writer, "Version:\t", build.Version)
		fmt.Fprintln(a.Writer, "Built time:\t", build.Time)
		fmt.Fprintln(a.Writer, "Built user:\t", build.User)
		return nil
	}

	f, err := os.OpenFile(a.Flags.LogFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()
	log.SetOutput(f)

	path := a.getPath()
	ui := a.createUI()

	if err := a.setNoCross(path); err != nil {
		return err
	}

	ui.SetIgnoreDirPaths(a.Flags.IgnoreDirs)

	if len(a.Flags.IgnoreDirPatterns) > 0 {
		if err := ui.SetIgnoreDirPatterns(a.Flags.IgnoreDirPatterns); err != nil {
			return err
		}
	}

	if a.Flags.NoHidden {
		ui.SetIgnoreHidden(true)
	}

	a.setMaxProcs()

	if err := a.runAction(ui, path); err != nil {
		return err
	}

	return ui.StartUILoop()
}

func (a *App) getPath() string {
	if len(a.Args) == 1 {
		return a.Args[0]
	}
	return "."
}

func (a *App) setMaxProcs() {
	if a.Flags.MaxCores < 1 || a.Flags.MaxCores > runtime.NumCPU() {
		return
	}

	runtime.GOMAXPROCS(a.Flags.MaxCores)

	// runtime.GOMAXPROCS(n) with n < 1 doesn't change current setting so we use it to check current value
	log.Printf("Max cores set to %d", runtime.GOMAXPROCS(0))
}

func (a *App) createUI() UI {
	var ui UI

	if a.Flags.NonInteractive || !a.Istty {
		ui = stdout.CreateStdoutUI(
			a.Writer,
			!a.Flags.NoColor && a.Istty,
			!a.Flags.NoProgress && a.Istty,
			a.Flags.ShowApparentSize,
		)
	} else {
		ui = tui.CreateUI(a.TermApp, !a.Flags.NoColor, a.Flags.ShowApparentSize)

		if !a.Flags.NoColor {
			tview.Styles.TitleColor = tcell.NewRGBColor(27, 161, 227)
		}
		tview.Styles.BorderColor = tcell.ColorDefault
	}
	return ui
}

func (a *App) setNoCross(path string) error {
	if a.Flags.NoCross {
		mounts, err := a.Getter.GetMounts()
		if err != nil {
			return fmt.Errorf("loading mount points: %w", err)
		}
		paths := device.GetNestedMountpointsPaths(path, mounts)
		a.Flags.IgnoreDirs = append(a.Flags.IgnoreDirs, paths...)
	}
	return nil
}

func (a *App) runAction(ui UI, path string) error {
	if a.Flags.ShowDisks {
		if err := ui.ListDevices(a.Getter); err != nil {
			return fmt.Errorf("loading mount points: %w", err)
		}
	} else {
		if err := ui.AnalyzePath(path, nil); err != nil {
			return fmt.Errorf("scanning dir: %w", err)
		}
	}
	return nil
}
