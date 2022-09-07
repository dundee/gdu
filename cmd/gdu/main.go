package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-isatty"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dundee/gdu/v5/cmd/gdu/app"
	"github.com/dundee/gdu/v5/pkg/device"
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
	flags.String("config-file", "", "Read config from file (default is $HOME/.gdu.yaml)")
	flags.StringP("log-file", "l", "/dev/null", "Path to a logfile")
	flags.StringP("output-file", "o", "", "Export all info into file as JSON")
	flags.StringP("input-file", "f", "", "Import analysis from JSON file")
	flags.IntP("max-cores", "m", runtime.NumCPU(), fmt.Sprintf("Set max cores that GDU will use. %d cores available", runtime.NumCPU()))
	flags.BoolP("version", "v", false, "Print version")

	flags.StringSliceP("ignore-dirs", "i", []string{"/proc", "/dev", "/sys", "/run"}, "Absolute paths to ignore (separated by comma)")
	flags.StringSliceP("ignore-dirs-pattern", "I", []string{}, "Absolute path patterns to ignore (separated by comma)")
	flags.StringP("ignore-from", "X", "", "Read absolute path patterns to ignore from file")
	flags.BoolP("no-hidden", "H", false, "Ignore hidden directories (beginning with dot)")
	flags.BoolP("no-cross", "x", false, "Do not cross filesystem boundaries")
	flags.BoolP("const-gc", "g", false, "Enable memory garbage collection during analysis with constant level set by GOGC")
	flags.Bool("enable-profiling", false, "Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/")

	flags.BoolP("show-disks", "d", false, "Show all mounted disks")
	flags.BoolP("show-apparent-size", "a", false, "Show apparent size")
	flags.BoolP("show-relative-size", "B", false, "Show relative size")
	flags.BoolP("no-color", "c", false, "Do not use colorized output")
	flags.BoolP("non-interactive", "n", false, "Do not run in interactive mode")
	flags.BoolP("no-progress", "p", false, "Do not show progress in non-interactive mode")
	flags.BoolP("summarize", "s", false, "Show only a total in non-interactive mode")
	flags.Bool("si", false, "Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)")
	flags.Bool("no-prefix", false, "Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode")
	flags.Bool("no-mouse", false, "Do not use mouse")
	flags.Bool("write-config", false, "Write current configuration to file (default is $HOME/.gdu.yaml)")

	err := viper.BindPFlags(flags)
	if err != nil {
		panic(err)
	}
}

func initConfig() error {
	if af.CfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(af.CfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".gdu" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gdu.yaml")
	}

	viper.AutomaticEnv()

	return viper.ReadInConfig()
}

func setAppFlags() {
	af.CfgFile = viper.GetString("config-file")
	af.LogFile = viper.GetString("log-file")
	af.OutputFile = viper.GetString("output-file")
	af.InputFile = viper.GetString("input-file")
	af.MaxCores = viper.GetInt("max-cores")
	af.ShowVersion = viper.GetBool("version")

	af.IgnoreDirs = viper.GetStringSlice("ignore-dirs")
	af.IgnoreDirPatterns = viper.GetStringSlice("ignore-dirs-pattern")
	af.IgnoreFromFile = viper.GetString("ignore-from")
	af.NoHidden = viper.GetBool("no-hidden")
	af.NoCross = viper.GetBool("no-cross")
	af.ConstGC = viper.GetBool("const-gc")
	af.Profiling = viper.GetBool("enable-profiling")

	af.ShowDisks = viper.GetBool("show-disks")
	af.ShowApparentSize = viper.GetBool("show-apparent-size")
	af.ShowRelativeSize = viper.GetBool("show-relative-size")
	af.NoColor = viper.GetBool("no-color")
	af.NonInteractive = viper.GetBool("non-interactive")
	af.NoProgress = viper.GetBool("no-progress")
	af.Summarize = viper.GetBool("summarize")
	af.UseSIPrefix = viper.GetBool("si")
	af.NoPrefix = viper.GetBool("no-prefix")
	af.NoMouse = viper.GetBool("no-mouse")
	af.WriteConfig = viper.GetBool("write-config")
}

func runE(command *cobra.Command, args []string) error {
	var (
		termApp *tview.Application
		screen  tcell.Screen
		err     error
	)

	coerr := initConfig()

	setAppFlags()

	if af.WriteConfig {
		err = viper.WriteConfig()
		if err != nil {
			return fmt.Errorf("Error writing config file: %w", err)
		}
	}

	var f *os.File
	if af.LogFile == "-" {
		f = os.Stdout
	} else {
		f, err = os.OpenFile(af.LogFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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

	if coerr != nil {
		log.Printf("Error reading config file: %s", coerr.Error())
	}

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
