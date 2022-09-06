package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/dundee/gdu/v5/cmd/gdu/app"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-isatty"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var af *app.Flags

var rootCmd = &cobra.Command{
	Use:   "gdu [directory_to_scan]",
	Short: "Pretty fast disk usage analyzer written in Go",
	Long: `Pretty fast disk usage analyzer written in Go.

Gdu is intended primarily for SSD disks where it can fully utilize parallel processing.
However HDDs work as well, but the performance gain is not so huge.
`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runE,
}

func init() {
	af = &app.Flags{}
	flags := rootCmd.Flags()
	flags.StringVarP(&af.LogFile, "log-file", "l", "/dev/null", "Path to a logfile")
	flags.StringVarP(&af.OutputFile, "output-file", "o", "", "Export all info into file as JSON")
	flags.StringVarP(&af.InputFile, "input-file", "f", "", "Import analysis from JSON file")
	flags.IntVarP(&af.MaxCores, "max-cores", "m", runtime.NumCPU(), fmt.Sprintf("Set max cores that GDU will use. %d cores available", runtime.NumCPU()))
	flags.BoolVarP(&af.ShowVersion, "version", "v", false, "Print version")

	flags.StringSliceVarP(&af.IgnoreDirs, "ignore-dirs", "i", []string{"/proc", "/dev", "/sys", "/run"}, "Absolute paths to ignore (separated by comma)")
	flags.StringSliceVarP(&af.IgnoreDirPatterns, "ignore-dirs-pattern", "I", []string{}, "Absolute path patterns to ignore (separated by comma)")
	flags.StringVarP(&af.IgnoreFromFile, "ignore-from", "X", "", "Read absolute path patterns to ignore from file")
	flags.BoolVarP(&af.NoHidden, "no-hidden", "H", false, "Ignore hidden directories (beginning with dot)")
	flags.BoolVarP(&af.NoCross, "no-cross", "x", false, "Do not cross filesystem boundaries")
	flags.BoolVarP(&af.ConstGC, "const-gc", "g", false, "Enable memory garbage collection during analysis with constant level set by GOGC")
	flags.BoolVar(&af.Profiling, "enable-profiling", false, "Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/")

	flags.BoolVarP(&af.ShowDisks, "show-disks", "d", false, "Show all mounted disks")
	flags.BoolVarP(&af.ShowApparentSize, "show-apparent-size", "a", false, "Show apparent size")
	flags.BoolVarP(&af.ShowRelativeSize, "show-relative-size", "B", false, "Show relative size")
	flags.BoolVarP(&af.NoColor, "no-color", "c", false, "Do not use colorized output")
	flags.BoolVarP(&af.NonInteractive, "non-interactive", "n", false, "Do not run in interactive mode")
	flags.BoolVarP(&af.NoProgress, "no-progress", "p", false, "Do not show progress in non-interactive mode")
	flags.BoolVarP(&af.Summarize, "summarize", "s", false, "Show only a total in non-interactive mode")
	flags.BoolVar(&af.UseSIPrefix, "si", false, "Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)")
	flags.BoolVar(&af.NoPrefix, "no-prefix", false, "Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode")
	flags.BoolVar(&af.NoMouse, "no-mouse", false, "Do not use mouse")
}

func runE(command *cobra.Command, args []string) error {
	var (
		termApp *tview.Application
		screen  tcell.Screen
		err     error
	)

	istty := isatty.IsTerminal(os.Stdout.Fd())

	// we are not able to analyze disk usage on Windows and Plan9
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		af.ShowApparentSize = true
	}
	if runtime.GOOS == "windows" && af.LogFile == "/dev/null" {
		af.LogFile = "nul"
	}

	if !af.ShowVersion && !af.NonInteractive && istty && af.OutputFile == "" {
		screen, err = tcell.NewScreen()
		if err != nil {
			return fmt.Errorf("Error creating screen: %w", err)
		}
		err = screen.Init()
		if err != nil {
			return fmt.Errorf("Error initializing screen: %w", err)
		}
		defer screen.Clear()
		defer screen.Fini()

		termApp = tview.NewApplication()
		termApp.SetScreen(screen)

		if !af.NoMouse {
			termApp.EnableMouse(true)
		}
	}

	a := app.App{
		Flags:       af,
		Args:        args,
		Istty:       istty,
		Writer:      os.Stdout,
		TermApp:     termApp,
		Screen:      screen,
		Getter:      device.Getter,
		PathChecker: os.Stat,
	}
	return a.Run()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
