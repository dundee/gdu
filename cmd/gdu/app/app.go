package app

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
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
	ReadFromStorage(storagePath, path string) error
	SetIgnoreDirPaths(paths []string)
	SetIgnoreDirPatterns(paths []string) error
	SetIgnoreFromFile(ignoreFile string) error
	SetIgnoreHidden(value bool)
	SetFollowSymlinks(value bool)
	SetAnalyzer(analyzer common.Analyzer)
	StartUILoop() error
}

// Flags define flags accepted by Run
type Flags struct {
	CfgFile            string   `yaml:"-"`
	LogFile            string   `yaml:"log-file"`
	InputFile          string   `yaml:"input-file"`
	OutputFile         string   `yaml:"output-file"`
	IgnoreDirs         []string `yaml:"ignore-dirs"`
	IgnoreDirPatterns  []string `yaml:"ignore-dir-patterns"`
	IgnoreFromFile     string   `yaml:"ignore-from-file"`
	MaxCores           int      `yaml:"max-cores"`
	SequentialScanning bool     `yaml:"sequential-scanning"`
	ShowDisks          bool     `yaml:"-"`
	ShowApparentSize   bool     `yaml:"show-apparent-size"`
	ShowRelativeSize   bool     `yaml:"show-relative-size"`
	ShowVersion        bool     `yaml:"-"`
	ShowItemCount      bool     `yaml:"show-item-count"`
	ShowMTime          bool     `yaml:"show-mtime"`
	NoColor            bool     `yaml:"no-color"`
	NoMouse            bool     `yaml:"no-mouse"`
	NonInteractive     bool     `yaml:"non-interactive"`
	NoProgress         bool     `yaml:"no-progress"`
	NoUnicode          bool     `yaml:"no-unicode"`
	NoCross            bool     `yaml:"no-cross"`
	NoHidden           bool     `yaml:"no-hidden"`
	NoDelete           bool     `yaml:"no-delete"`
	FollowSymlinks     bool     `yaml:"follow-symlinks"`
	Profiling          bool     `yaml:"profiling"`
	ConstGC            bool     `yaml:"const-gc"`
	UseStorage         bool     `yaml:"use-storage"`
	StoragePath        string   `yaml:"storage-path"`
	ReadFromStorage    bool     `yaml:"read-from-storage"`
	Summarize          bool     `yaml:"summarize"`
	Top                int      `yaml:"top"`
	UseSIPrefix        bool     `yaml:"use-si-prefix"`
	NoPrefix           bool     `yaml:"no-prefix"`
	WriteConfig        bool     `yaml:"-"`
	ChangeCwd          bool     `yaml:"change-cwd"`
	DeleteInBackground bool     `yaml:"delete-in-background"`
	DeleteInParallel   bool     `yaml:"delete-in-parallel"`
	Style              Style    `yaml:"style"`
	Sorting            Sorting  `yaml:"sorting"`
}

// Style define style config
type Style struct {
	SelectedRow   ColorStyle          `yaml:"selected-row"`
	ProgressModal ProgressModalOpts   `yaml:"progress-modal"`
	UseOldSizeBar bool                `yaml:"use-old-size-bar"`
	Footer        FooterColorStyle    `yaml:"footer"`
	Header        HeaderColorStyle    `yaml:"header"`
	ResultRow     ResultRowColorStyle `yaml:"result-row"`
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

// FooterColorStyle defines styling of footer
type FooterColorStyle struct {
	TextColor       string `yaml:"text-color"`
	BackgroundColor string `yaml:"background-color"`
	NumberColor     string `yaml:"number-color"`
}

// HeaderColorStyle defines styling of header
type HeaderColorStyle struct {
	TextColor       string `yaml:"text-color"`
	BackgroundColor string `yaml:"background-color"`
	Hidden          bool   `yaml:"hidden"`
}

// ResultRowColorStyle defines styling of result row
type ResultRowColorStyle struct {
	NumberColor    string `yaml:"number-color"`
	DirectoryColor string `yaml:"directory-color"`
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
func (a *App) Run() error {
	var ui UI

	if a.Flags.ShowVersion {
		fmt.Fprintln(a.Writer, "Version:\t", build.Version)
		fmt.Fprintln(a.Writer, "Built time:\t", build.Time)
		fmt.Fprintln(a.Writer, "Built user:\t", build.User)
		return nil
	}

	log.Printf("Runtime flags: %+v", *a.Flags)

	if a.Flags.NoPrefix && a.Flags.UseSIPrefix {
		return fmt.Errorf("--no-prefix and --si cannot be used at once")
	}

	path := a.getPath()
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	ui, err = a.createUI()
	if err != nil {
		return err
	}

	if a.Flags.UseStorage {
		ui.SetAnalyzer(analyze.CreateStoredAnalyzer(a.Flags.StoragePath))
	}
	if a.Flags.SequentialScanning {
		ui.SetAnalyzer(analyze.CreateSeqAnalyzer())
	}
	if a.Flags.FollowSymlinks {
		ui.SetFollowSymlinks(true)
	}
	if err := a.setNoCross(path); err != nil {
		return err
	}

	ui.SetIgnoreDirPaths(a.Flags.IgnoreDirs)

	if len(a.Flags.IgnoreDirPatterns) > 0 {
		if err := ui.SetIgnoreDirPatterns(a.Flags.IgnoreDirPatterns); err != nil {
			return err
		}
	}

	if a.Flags.IgnoreFromFile != "" {
		if err := ui.SetIgnoreFromFile(a.Flags.IgnoreFromFile); err != nil {
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

func (a *App) createUI() (UI, error) {
	var ui UI

	switch {
	case a.Flags.OutputFile != "":
		var output io.Writer
		var err error
		if a.Flags.OutputFile == "-" {
			output = os.Stdout
		} else {
			output, err = os.OpenFile(a.Flags.OutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
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
	case a.Flags.NonInteractive || !a.Istty:
		stdoutUI := stdout.CreateStdoutUI(
			a.Writer,
			!a.Flags.NoColor && a.Istty,
			!a.Flags.NoProgress && a.Istty,
			a.Flags.ShowApparentSize,
			a.Flags.ShowRelativeSize,
			a.Flags.Summarize,
			a.Flags.ConstGC,
			a.Flags.UseSIPrefix,
			a.Flags.NoPrefix,
			a.Flags.Top,
		)
		if a.Flags.NoUnicode {
			stdoutUI.UseOldProgressRunes()
		}
		ui = stdoutUI
	default:
		opts := a.getOptions()

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

	return ui, nil
}

func (a *App) getOptions() []tui.Option {
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
	if a.Flags.Style.Footer.TextColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetFooterTextColor(a.Flags.Style.Footer.TextColor)
		})
	}
	if a.Flags.Style.Footer.BackgroundColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetFooterBackgroundColor(a.Flags.Style.Footer.BackgroundColor)
		})
	}
	if a.Flags.Style.Footer.NumberColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetFooterNumberColor(a.Flags.Style.Footer.NumberColor)
		})
	}
	if a.Flags.Style.Header.TextColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetHeaderTextColor(a.Flags.Style.Header.TextColor)
		})
	}
	if a.Flags.Style.Header.BackgroundColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetHeaderBackgroundColor(a.Flags.Style.Header.BackgroundColor)
		})
	}
	if a.Flags.Style.Header.Hidden {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetHeaderHidden()
		})
	}
	if a.Flags.Style.ResultRow.NumberColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetResultRowNumberColor(a.Flags.Style.ResultRow.NumberColor)
		})
	}
	if a.Flags.Style.ResultRow.DirectoryColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetResultRowDirectoryColor(a.Flags.Style.ResultRow.DirectoryColor)
		})
	}
	if a.Flags.Style.ProgressModal.CurrentItemNameMaxLen > 0 {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetCurrentItemNameMaxLen(a.Flags.Style.ProgressModal.CurrentItemNameMaxLen)
		})
	}
	if a.Flags.Style.UseOldSizeBar || a.Flags.NoUnicode {
		opts = append(opts, func(ui *tui.UI) {
			ui.UseOldSizeBar()
		})
	}
	if a.Flags.Sorting.Order != "" || a.Flags.Sorting.By != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetDefaultSorting(a.Flags.Sorting.By, a.Flags.Sorting.Order)
		})
	}
	if a.Flags.ChangeCwd {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetChangeCwdFn(os.Chdir)
		})
	}
	if a.Flags.ShowItemCount {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetShowItemCount()
		})
	}
	if a.Flags.ShowMTime {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetShowMTime()
		})
	}
	if a.Flags.NoDelete {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetNoDelete()
		})
	}
	if a.Flags.DeleteInBackground {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetDeleteInBackground()
		})
	}
	if a.Flags.DeleteInParallel {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetDeleteInParallel()
		})
	}
	return opts
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

	switch {
	case a.Flags.ShowDisks:
		if err := ui.ListDevices(a.Getter); err != nil {
			return fmt.Errorf("loading mount points: %w", err)
		}
	case a.Flags.InputFile != "":
		var input io.Reader
		var err error
		if a.Flags.InputFile == "-" {
			input = os.Stdin
		} else {
			input, err = os.OpenFile(a.Flags.InputFile, os.O_RDONLY, 0o600)
			if err != nil {
				return fmt.Errorf("opening input file: %w", err)
			}
		}

		if err := ui.ReadAnalysis(input); err != nil {
			return fmt.Errorf("reading analysis: %w", err)
		}
	case a.Flags.ReadFromStorage:
		ui.SetAnalyzer(analyze.CreateStoredAnalyzer(a.Flags.StoragePath))
		if err := ui.ReadFromStorage(a.Flags.StoragePath, path); err != nil {
			return fmt.Errorf("reading from storage (%s): %w", a.Flags.StoragePath, err)
		}
	default:
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
