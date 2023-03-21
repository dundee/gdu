package app

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"net/http"
	"net/http/pprof"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/device"
	gfs "github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/report"
	"github.com/dundee/gdu/v5/stdout"
	"github.com/dundee/gdu/v5/tui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI is common interface for both terminal UI and text output
type UI interface {
	ListDevices(getter device.DevicesInfoGetter) error
	AnalyzePath(path string, parentDir gfs.Item) error
	ReadAnalysis(input io.Reader) error
	SetIgnoreDirPaths(paths []string)
	SetIgnoreDirPatterns(paths []string) error
	SetIgnoreFromFile(ignoreFile string) error
	SetIgnoreHidden(value bool)
	SetFollowSymlinks(value bool)
	StartUILoop() error
}

// Flags define flags accepted by Run
type Flags struct {
	CfgFile           string   `yaml:"-"`
	LogFile           string   `yaml:"log-file"`
	InputFile         string   `yaml:"input-file"`
	OutputFile        string   `yaml:"output-file"`
	IgnoreDirs        []string `yaml:"ignore-dirs"`
	IgnoreDirPatterns []string `yaml:"ignore-dir-patterns"`
	IgnoreFromFile    string   `yaml:"ignore-from-file"`
	MaxCores          int      `yaml:"max-cores"`
	ShowDisks         bool     `yaml:"-"`
	ShowApparentSize  bool     `yaml:"show-apparent-size"`
	ShowRelativeSize  bool     `yaml:"show-relative-size"`
	ShowVersion       bool     `yaml:"-"`
	NoColor           bool     `yaml:"no-color"`
	NoMouse           bool     `yaml:"no-mouse"`
	NonInteractive    bool     `yaml:"non-interactive"`
	NoProgress        bool     `yaml:"no-progress"`
	NoCross           bool     `yaml:"no-cross"`
	NoHidden          bool     `yaml:"no-hidden"`
	FollowSymlinks    bool     `yaml:"follow-symlinks"`
	Profiling         bool     `yaml:"profiling"`
	ConstGC           bool     `yaml:"const-gc"`
	Summarize         bool     `yaml:"summarize"`
	UseSIPrefix       bool     `yaml:"use-si-prefix"`
	NoPrefix          bool     `yaml:"no-prefix"`
	WriteConfig       bool     `yaml:"-"`
	ChangeCwd         bool     `yaml:"change-cwd"`
	Style             Style    `yaml:"style"`
	Sorting           Sorting  `yaml:"sorting"`
}

// Style define style config
type Style struct {
	SelectedRow   ColorStyle        `yaml:"selected-row"`
	ProgressModal ProgressModalOpts `yaml:"progress-modal"`
}

// ProgressModalOpts defines options for progress modal
type ProgressModalOpts struct {
	CurrentItemNameMaxLen int `yaml:"current-item-path-max-len"`
}

// ColorStyle defines styling of some item
type ColorStyle struct {
	TextColor       string `yaml:"text-color"`
	BackgroundColor string `yaml:"background-color"`
}

// Sorting defines default sorting of items
type Sorting struct {
	By    string `yaml:"by"`
	Order string `yaml:"order"`
}

// App defines the main application
type App struct {
	Args        []string
	Flags       *Flags
	Istty       bool
	Writer      io.Writer
	TermApp     common.TermApplication
	Screen      tcell.Screen
	Getter      device.DevicesInfoGetter
	PathChecker func(string) (fs.FileInfo, error)
}

func init() {
	http.DefaultServeMux = http.NewServeMux()
}

// Run starts gdu main logic
func (a *App) Run() (err error) {
	var ui UI

	if a.Flags.ShowVersion {
		fmt.Fprintln(a.Writer, "Version:\t", build.Version)
		fmt.Fprintln(a.Writer, "Built time:\t", build.Time)
		fmt.Fprintln(a.Writer, "Built user:\t", build.User)
		return
	}

	log.Printf("Runtime flags: %+v", *a.Flags)

	if a.Flags.NoPrefix && a.Flags.UseSIPrefix {
		return fmt.Errorf("--no-prefix and --si cannot be used at once")
	}

	path := a.getPath()
	path, _ = filepath.Abs(path)

	ui, err = a.createUI()
	if err != nil {
		return
	}

	if err = a.setNoCross(path); err != nil {
		return
	}

	ui.SetIgnoreDirPaths(a.Flags.IgnoreDirs)

	if len(a.Flags.IgnoreDirPatterns) > 0 {
		if err = ui.SetIgnoreDirPatterns(a.Flags.IgnoreDirPatterns); err != nil {
			return
		}
	}

	if a.Flags.IgnoreFromFile != "" {
		if err = ui.SetIgnoreFromFile(a.Flags.IgnoreFromFile); err != nil {
			return
		}
	}

	if a.Flags.NoHidden {
		ui.SetIgnoreHidden(true)
	}

	a.setMaxProcs()

	if err = a.runAction(ui, path); err != nil {
		return
	}

	err = ui.StartUILoop()
	return
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

func (a *App) createUI() (UI, error) {
	var ui UI

	if a.Flags.OutputFile != "" {
		var output io.Writer
		var err error
		if a.Flags.OutputFile == "-" {
			output = os.Stdout
		} else {
			output, err = os.OpenFile(a.Flags.OutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				return nil, fmt.Errorf("opening output file: %w", err)
			}
		}
		ui = report.CreateExportUI(
			a.Writer,
			output,
			!a.Flags.NoColor && a.Istty,
			!a.Flags.NoProgress && a.Istty,
			a.Flags.ConstGC,
			a.Flags.UseSIPrefix,
		)
	} else if a.Flags.NonInteractive || !a.Istty {
		ui = stdout.CreateStdoutUI(
			a.Writer,
			!a.Flags.NoColor && a.Istty,
			!a.Flags.NoProgress && a.Istty,
			a.Flags.ShowApparentSize,
			a.Flags.ShowRelativeSize,
			a.Flags.Summarize,
			a.Flags.ConstGC,
			a.Flags.UseSIPrefix,
			a.Flags.NoPrefix,
		)
	} else {
		var opts []tui.Option

		if a.Flags.Style.SelectedRow.TextColor != "" {
			opts = append(opts, func(ui *tui.UI) {
				ui.SetSelectedTextColor(tcell.GetColor(a.Flags.Style.SelectedRow.TextColor))
			})
		}
		if a.Flags.Style.SelectedRow.BackgroundColor != "" {
			opts = append(opts, func(ui *tui.UI) {
				ui.SetSelectedBackgroundColor(tcell.GetColor(a.Flags.Style.SelectedRow.BackgroundColor))
			})
		}
		if a.Flags.Style.ProgressModal.CurrentItemNameMaxLen > 0 {
			opts = append(opts, func(ui *tui.UI) {
				ui.SetCurrentItemNameMaxLen(a.Flags.Style.ProgressModal.CurrentItemNameMaxLen)
			})
		}
		if a.Flags.Sorting.Order != "" || a.Flags.Sorting.By != "" {
			opts = append(opts, func(ui *tui.UI) {
				ui.SetDefaultSorting(a.Flags.Sorting.By, a.Flags.Sorting.Order)
			})
		}
		if a.Flags.ChangeCwd != false {
			opts = append(opts, func(ui *tui.UI) {
				ui.SetChangeCwdFn(os.Chdir)
			})
		}

		ui = tui.CreateUI(
			a.TermApp,
			a.Screen,
			os.Stdout,
			!a.Flags.NoColor,
			a.Flags.ShowApparentSize,
			a.Flags.ShowRelativeSize,
			a.Flags.ConstGC,
			a.Flags.UseSIPrefix,
			opts...,
		)

		if !a.Flags.NoColor {
			tview.Styles.TitleColor = tcell.NewRGBColor(27, 161, 227)
		} else {
			tview.Styles.ContrastBackgroundColor = tcell.NewRGBColor(150, 150, 150)
		}
		tview.Styles.BorderColor = tcell.ColorDefault
	}

	if a.Flags.FollowSymlinks {
		ui.SetFollowSymlinks(true)
	}

	return ui, nil
}

func (a *App) setNoCross(path string) error {
	if a.Flags.NoCross {
		mounts, err := a.Getter.GetMounts()
		if err != nil {
			return fmt.Errorf("loading mount points: %w", err)
		}
		paths := device.GetNestedMountpointsPaths(path, mounts)
		log.Printf("Ignoring mount points: %s", strings.Join(paths, ", "))
		a.Flags.IgnoreDirs = append(a.Flags.IgnoreDirs, paths...)
	}
	return nil
}

func (a *App) runAction(ui UI, path string) error {
	if a.Flags.Profiling {
		go func() {
			http.HandleFunc("/debug/pprof/", pprof.Index)
			http.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			http.HandleFunc("/debug/pprof/profile", pprof.Profile)
			http.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			http.HandleFunc("/debug/pprof/trace", pprof.Trace)
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	if a.Flags.ShowDisks {
		if err := ui.ListDevices(a.Getter); err != nil {
			return fmt.Errorf("loading mount points: %w", err)
		}
	} else if a.Flags.InputFile != "" {
		var input io.Reader
		var err error
		if a.Flags.InputFile == "-" {
			input = os.Stdin
		} else {
			input, err = os.OpenFile(a.Flags.InputFile, os.O_RDONLY, 0600)
			if err != nil {
				return fmt.Errorf("opening input file: %w", err)
			}
		}

		if err := ui.ReadAnalysis(input); err != nil {
			return fmt.Errorf("reading analysis: %w", err)
		}
	} else {
		if build.RootPathPrefix != "" {
			path = build.RootPathPrefix + path
		}

		_, err := a.PathChecker(path)
		if err != nil {
			return err
		}

		log.Printf("Analyzing path: %s", path)
		if err := ui.AnalyzePath(path, nil); err != nil {
			return fmt.Errorf("scanning dir: %w", err)
		}
	}
	return nil
}
