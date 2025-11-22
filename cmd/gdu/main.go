package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-isatty"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/dundee/gdu/v5/cmd/gdu/app"
	"github.com/dundee/gdu/v5/pkg/device"
)

var (
	af        *app.Flags
	configErr error
)

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
	flags.StringVar(&af.CfgFile, "config-file", "", "Read config from file (default is $HOME/.gdu.yaml)")
	flags.StringVarP(&af.LogFile, "log-file", "l", "/dev/null", "Path to a logfile")
	flags.StringVarP(&af.OutputFile, "output-file", "o", "", "Export all info into file as JSON")
	flags.StringVarP(&af.InputFile, "input-file", "f", "", "Import analysis from JSON file")
	flags.IntVarP(&af.MaxCores, "max-cores", "m", runtime.NumCPU(), fmt.Sprintf("Set max cores that Gdu will use. %d cores available", runtime.NumCPU()))
	flags.BoolVar(&af.SequentialScanning, "sequential", false, "Use sequential scanning (intended for rotating HDDs)")
	flags.BoolVarP(&af.ShowVersion, "version", "v", false, "Print version")

	flags.StringSliceVarP(&af.IgnoreDirs, "ignore-dirs", "i", []string{"/proc", "/dev", "/sys", "/run"},
		"Paths to ignore (separated by comma). Can be absolute or relative to current directory")
	flags.StringSliceVarP(&af.IgnoreDirPatterns, "ignore-dirs-pattern", "I", []string{},
		"Path patterns to ignore (separated by comma)")
	flags.StringVarP(&af.IgnoreFromFile, "ignore-from", "X", "",
		"Read path patterns to ignore from file")
	flags.BoolVarP(&af.NoHidden, "no-hidden", "H", false, "Ignore hidden directories (beginning with dot)")
	flags.BoolVarP(
		&af.FollowSymlinks, "follow-symlinks", "L", false,
		"Follow symlinks for files, i.e. show the size of the file to which symlink points to (symlinks to directories are not followed)",
	)
	flags.BoolVarP(
		&af.ShowAnnexedSize, "show-annexed-size", "A", false,
		"Use apparent size of git-annex'ed files in case files are not present locally (real usage is zero)",
	)
	flags.BoolVarP(&af.NoCross, "no-cross", "x", false, "Do not cross filesystem boundaries")
	flags.BoolVarP(&af.ConstGC, "const-gc", "g", false, "Enable memory garbage collection during analysis with constant level set by GOGC")
	flags.BoolVar(&af.Profiling, "enable-profiling", false, "Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/")

	flags.BoolVar(&af.UseStorage, "use-storage", false, "Use persistent key-value storage for analysis data (experimental)")
	flags.StringVar(&af.StoragePath, "storage-path", "/tmp/badger", "Path to persistent key-value storage directory")
	flags.BoolVarP(&af.ReadFromStorage, "read-from-storage", "r", false, "Read analysis data from persistent key-value storage")

	flags.BoolVarP(&af.ShowDisks, "show-disks", "d", false, "Show all mounted disks")
	flags.BoolVarP(&af.ShowApparentSize, "show-apparent-size", "a", false, "Show apparent size")
	flags.BoolVarP(&af.ShowRelativeSize, "show-relative-size", "B", false, "Show relative size")
	flags.BoolVarP(&af.NoColor, "no-color", "c", false, "Do not use colorized output")
	flags.BoolVarP(&af.ShowItemCount, "show-item-count", "C", false, "Show number of items in directory")
	flags.BoolVarP(&af.ShowMTime, "show-mtime", "M", false, "Show latest mtime of items in directory")
	flags.BoolVarP(&af.NonInteractive, "non-interactive", "n", false, "Do not run in interactive mode")
	flags.BoolVarP(&af.NoProgress, "no-progress", "p", false, "Do not show progress in non-interactive mode")
	flags.BoolVarP(&af.NoUnicode, "no-unicode", "u", false, "Do not use Unicode symbols (for size bar)")
	flags.BoolVarP(&af.Summarize, "summarize", "s", false, "Show only a total in non-interactive mode")
	flags.IntVarP(&af.Top, "top", "t", 0, "Show only top X largest files in non-interactive mode")
	flags.BoolVar(&af.UseSIPrefix, "si", false, "Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)")
	flags.BoolVar(&af.NoPrefix, "no-prefix", false, "Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode")
	flags.BoolVar(&af.ReverseSort, "reverse-sort", false, "Reverse sorting order (smallest to largest) in non-interactive mode")
	flags.BoolVar(&af.Mouse, "mouse", false, "Use mouse")
	flags.BoolVar(&af.NoDelete, "no-delete", false, "Do not allow deletions")
	flags.BoolVar(&af.NoSpawnShell, "no-spawn-shell", false, "Do not allow spawning shell")
	flags.BoolVar(&af.WriteConfig, "write-config", false, "Write current configuration to file (default is $HOME/.gdu.yaml)")
	flags.StringVar(
		&af.Since, "since", "",
		"Include files with mtime >= WHEN. WHEN accepts RFC3339 timestamp (e.g., 2025-08-11T01:00:00-07:00) "+
			"or date only YYYY-MM-DD (calendar-day compare; includes the whole day)",
	)
	flags.StringVar(&af.Until, "until", "", "Include files with mtime <= WHEN. WHEN accepts RFC3339 timestamp or date only YYYY-MM-DD")
	flags.StringVar(&af.MaxAge, "max-age", "", "Include files with mtime no older than DURATION (e.g., 7d, 2h30m, 1y2mo)")
	flags.StringVar(&af.MinAge, "min-age", "", "Include files with mtime at least DURATION old (e.g., 30d, 1w)")

	initConfig()
	setDefaults()
}

func initConfig() {
	setConfigFilePath()
	data, err := os.ReadFile(af.CfgFile)
	if err != nil {
		configErr = err
		return // config file does not exist, return
	}

	configErr = yaml.Unmarshal(data, &af)
}

func setDefaults() {
	if af.Style.Footer.BackgroundColor == "" {
		af.Style.Footer.BackgroundColor = "#2479D0"
	}
	if af.Style.Footer.TextColor == "" {
		af.Style.Footer.TextColor = "#000000"
	}
	if af.Style.Footer.NumberColor == "" {
		af.Style.Footer.NumberColor = "#FFFFFF"
	}
	if af.Style.Header.BackgroundColor == "" {
		af.Style.Header.BackgroundColor = "#2479D0"
	}
	if af.Style.Header.TextColor == "" {
		af.Style.Header.TextColor = "#000000"
	}
	if af.Style.ResultRow.NumberColor == "" {
		af.Style.ResultRow.NumberColor = "#e67100"
	}
	if af.Style.ResultRow.DirectoryColor == "" {
		af.Style.ResultRow.DirectoryColor = "#3498db"
	}
}

func setConfigFilePath() {
	command := strings.Join(os.Args, " ")
	if strings.Contains(command, "--config-file") {
		re := regexp.MustCompile("--config-file[= ]([^ ]+)")
		parts := re.FindStringSubmatch(command)

		if len(parts) > 1 {
			af.CfgFile = parts[1]
			return
		}
	}
	setDefaultConfigFilePath()
}

func setDefaultConfigFilePath() {
	home, err := os.UserHomeDir()
	if err != nil {
		configErr = err
		return
	}

	path := filepath.Join(home, ".config", "gdu", "gdu.yaml")
	if _, err := os.Stat(path); err == nil {
		af.CfgFile = path
		return
	}

	af.CfgFile = filepath.Join(home, ".gdu.yaml")
}

func runE(command *cobra.Command, args []string) error {
	var (
		termApp *tview.Application
		screen  tcell.Screen
		err     error
	)

	if af.WriteConfig {
		data, err := yaml.Marshal(af)
		if err != nil {
			return fmt.Errorf("error marshaling config file: %w", err)
		}
		if af.CfgFile == "" {
			setDefaultConfigFilePath()
		}
		err = os.WriteFile(af.CfgFile, data, 0o600)
		if err != nil {
			return fmt.Errorf("error writing config file %s: %w", af.CfgFile, err)
		}
	}

	if runtime.GOOS == "windows" && af.LogFile == "/dev/null" {
		af.LogFile = "nul"
	}

	var f *os.File
	if af.LogFile == "-" {
		f = os.Stdout
	} else {
		f, err = os.OpenFile(af.LogFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer func() {
			cerr := f.Close()
			if cerr != nil {
				panic(cerr)
			}
		}()
	}
	log.SetOutput(f)

	if configErr != nil {
		log.Printf("Error reading config file: %s", configErr.Error())
	}

	istty := isatty.IsTerminal(os.Stdout.Fd())

	// we are not able to analyze disk usage on Windows and Plan9
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		af.ShowApparentSize = true
	}

	if !af.ShouldRunInNonInteractiveMode(istty) {
		screen, err = tcell.NewScreen()
		if err != nil {
			return fmt.Errorf("error creating screen: %w", err)
		}
		defer screen.Clear()
		defer screen.Fini()

		termApp = tview.NewApplication()
		termApp.SetScreen(screen)

		if af.Mouse {
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
