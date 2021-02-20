package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/dundee/gdu/cmd/run"
	"github.com/dundee/gdu/device"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-isatty"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var rf *run.Flags

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
	rf = &run.Flags{}
	flags := rootCmd.Flags()
	flags.StringVarP(&rf.LogFile, "log-file", "l", "/dev/null", "Path to a logfile")
	flags.StringSliceVarP(&rf.IgnoreDirs, "ignore-dirs", "i", []string{"/proc", "/dev", "/sys", "/run"}, "Absolute paths to ignore (separated by comma)")
	flags.BoolVarP(&rf.ShowDisks, "show-disks", "d", false, "Show all mounted disks")
	flags.BoolVarP(&rf.ShowApparentSize, "show-apparent-size", "a", false, "Show apparent size")
	flags.BoolVarP(&rf.ShowVersion, "version", "v", false, "Print version")
	flags.BoolVarP(&rf.NoColor, "no-color", "c", false, "Do not use colorized output")
	flags.BoolVarP(&rf.NonInteractive, "non-interactive", "n", false, "Do not run in interactive mode")
	flags.BoolVarP(&rf.NoProgress, "no-progress", "p", false, "Do not show progress in non-interactive mode")
	flags.BoolVarP(&rf.NoCross, "no-cross", "x", false, "Do not cross filesystem boundaries")
}

func runE(command *cobra.Command, args []string) error {
	istty := isatty.IsTerminal(os.Stdout.Fd())

	// we are not able to analyze disk usage on Windows and Plan9
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		rf.ShowApparentSize = true
	}
	if runtime.GOOS == "windows" && rf.LogFile == "/dev/null" {
		rf.LogFile = "nul"
	}

	var app *tview.Application = nil

	if !rf.ShowVersion && !rf.NonInteractive && istty {
		screen, err := tcell.NewScreen()
		if err != nil {
			return fmt.Errorf("Error creating screen: %w", err)
		}
		screen.Init()
		defer screen.Clear()
		defer screen.Fini()

		app = tview.NewApplication()
		app.SetScreen(screen)
	}

	return run.Run(rf, args, istty, os.Stdout, app, device.Getter)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
