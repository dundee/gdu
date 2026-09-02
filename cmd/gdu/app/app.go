package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	gfs "github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/pkg/timefilter"
	"github.com/dundee/gdu/v5/report"
	"github.com/dundee/gdu/v5/stdout"
	"github.com/dundee/gdu/v5/tui"
	"github.com/dundee/gdu/v5/webui"
)

// UI is common interface for both terminal UI and text output
type UI interface {
	ListDevices(getter device.DevicesInfoGetter) error
	AnalyzePath(path string, parentDir gfs.Item) error
	// AnalyzePaths scans several roots and presents them under one virtual top
	// level dir. Implementations that cannot represent more than one root
	// return an error.
	AnalyzePaths(paths []string) error
	ReadAnalysis(input io.Reader) error
	ReadFromStorage(storagePath, path string) error
	SetIgnoreTypes(types []string)
	SetIgnoreDirPaths(paths []string)
	SetIgnoreDirPatterns(paths []string) error
	SetIgnoreFromFile(ignoreFile string) error
	SetIgnoreHidden(value bool)
	SetIncludeTypes(types []string)
	SetFollowSymlinks(value bool)
	SetShowAnnexedSize(value bool)
	SetAnalyzer(analyzer common.Analyzer)
	SetTimeFilter(timeFilter common.TimeFilter)
	SetArchiveBrowsing(value bool)
	SetCollapsePath(value bool)
	SetShowSymlinkTarget(value bool)
	StartUILoop() error
}

// Flags define flags accepted by Run
type Flags struct {
	Style              Style     `yaml:"style"`
	Sorting            Sorting   `yaml:"sorting"`
	CfgFile            string    `yaml:"-"`
	LogFile            string    `yaml:"log-file"`
	InputFile          string    `yaml:"input-file"`
	OutputFile         string    `yaml:"output-file"`
	OutputAttrs        string    `yaml:"output-attrs"`
	IgnoreFromFile     string    `yaml:"ignore-from-file"`
	IgnoreDirs         []string  `yaml:"ignore-dirs"`
	IgnoreDirPatterns  []string  `yaml:"ignore-dir-patterns"`
	TypeFilter         []string  `yaml:"type"`
	ExcludeTypeFilter  []string  `yaml:"exclude-type"`
	MaxCores           int       `yaml:"max-cores"`
	Top                int       `yaml:"top"`
	Depth              int       `yaml:"depth"`
	SequentialScanning bool      `yaml:"sequential-scanning"`
	ShowDisks          bool      `yaml:"-"`
	ShowApparentSize   bool      `yaml:"show-apparent-size"`
	ShowRelativeSize   bool      `yaml:"show-relative-size"`
	ShowAnnexedSize    bool      `yaml:"show-annexed-size"`
	ShowVersion        bool      `yaml:"-"`
	ShowItemCount      bool      `yaml:"show-item-count"`
	ShowMTime          bool      `yaml:"show-mtime"`
	NoColor            bool      `yaml:"no-color"`
	Mouse              bool      `yaml:"mouse"`
	NonInteractive     bool      `yaml:"non-interactive"`
	Interactive        bool      `yaml:"interactive"`
	NoProgress         bool      `yaml:"no-progress"`
	NoUnicode          bool      `yaml:"no-unicode"`
	NoCross            bool      `yaml:"no-cross"`
	NoHidden           bool      `yaml:"no-hidden"`
	NoDelete           bool      `yaml:"no-delete"`
	NoViewFile         bool      `yaml:"no-view-file"`
	NoSpawnShell       bool      `yaml:"no-spawn-shell"`
	NoConfirmQuit      bool      `yaml:"no-confirm-quit"`
	FollowSymlinks     bool      `yaml:"follow-symlinks"`
	Profiling          bool      `yaml:"profiling"`
	ReadFromStorage    bool      `yaml:"read-from-storage"`
	DbPath             string    `yaml:"db"`
	Summarize          bool      `yaml:"summarize"`
	UseSIPrefix        bool      `yaml:"use-si-prefix"`
	NoPrefix           bool      `yaml:"no-prefix"`
	ShowInKiB          bool      `yaml:"show-in-kib"`
	WriteConfig        bool      `yaml:"-"`
	ReverseSort        bool      `yaml:"reverse-sort"`
	ChangeCwd          bool      `yaml:"change-cwd"`
	DeleteInBackground bool      `yaml:"delete-in-background"`
	DeleteInParallel   bool      `yaml:"delete-in-parallel"`
	Since              string    `yaml:"since"`
	Until              string    `yaml:"until"`
	MaxAge             string    `yaml:"max-age"`
	MinAge             string    `yaml:"min-age"`
	ArchiveBrowsing    bool      `yaml:"archive-browsing"`
	CollapsePath       bool      `yaml:"collapse-path"`
	ShowSymlinkTarget  bool      `yaml:"show-symlink-target"`
	BrowseParentDirs   bool      `yaml:"browse-parent-dirs"`
	Web                bool      `yaml:"-"`
	WebConfig          WebConfig `yaml:"web"`
}

// WebConfig defines the web UI options that can be set from the config file.
type WebConfig struct {
	Listen      string `yaml:"listen"`
	OpenBrowser bool   `yaml:"open-browser"`
	Browser     string `yaml:"browser"`
}

// ShouldRunInNonInteractiveMode checks if the application should run in non-interactive mode
// based on the flags set.
func (f *Flags) ShouldRunInNonInteractiveMode(istty bool) bool {
	if f.NonInteractive {
		return true
	}

	if f.Interactive {
		return f.ShowVersion ||
			f.OutputFile != "" ||
			f.NoPrefix ||
			f.NoProgress ||
			f.Summarize ||
			f.Top > 0
	}

	return !istty ||
		f.ShowVersion ||
		f.OutputFile != "" ||
		f.NoPrefix ||
		f.NoProgress ||
		f.Summarize ||
		f.Top > 0
}

// Style define style config
type Style struct {
	Footer        FooterColorStyle    `yaml:"footer"`
	SelectedRow   ColorStyle          `yaml:"selected-row"`
	Marked        ColorStyle          `yaml:"marked"`
	ResultRow     ResultRowColorStyle `yaml:"result-row"`
	Header        HeaderColorStyle    `yaml:"header"`
	ProgressModal ProgressModalOpts   `yaml:"progress-modal"`
	UseOldSizeBar bool                `yaml:"use-old-size-bar"`
	// ShowBarPercentage shows the numeric usage percentage next to the size bar.
	ShowBarPercentage bool `yaml:"show-bar-percentage"`
	// ShowItemCountBar shows a bar next to the item count column. It only takes
	// effect while the item count column is visible.
	ShowItemCountBar bool `yaml:"show-item-count-bar"`
}

// ProgressModalOpts defines options for progress modal
type ProgressModalOpts struct {
	CurrentItemNameMaxLen int  `yaml:"current-item-path-max-len"`
	ShowDiskProgressBar   bool `yaml:"show-disk-progress-bar"`
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
	Writer      io.Writer
	TermApp     common.TermApplication
	Screen      tcell.Screen
	Getter      device.DevicesInfoGetter
	Flags       *Flags
	PathChecker func(string) (fs.FileInfo, error)
	Args        []string
	Istty       bool
}

func init() {
	http.DefaultServeMux = http.NewServeMux()
}

// Run starts gdu main logic
//
//nolint:gocyclo,funlen // App function is a suite of if statements
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

	if a.Flags.NonInteractive && a.Flags.Interactive {
		return fmt.Errorf("--interactive and --non-interactive cannot be used at once")
	}

	outputAttributes, err := parseJSONAttributes(a.Flags.OutputAttrs)
	if err != nil {
		return err
	}
	if a.Flags.OutputAttrs != "" && a.Flags.OutputFile == "" {
		return errors.New("--output-attrs requires --output-file")
	}

	paths, err := a.resolvePaths()
	if err != nil {
		return err
	}
	if len(paths) > 1 {
		if err := a.checkMultiPathSupport(); err != nil {
			return err
		}
	}

	ui, err = a.createUI(outputAttributes)
	if err != nil {
		return err
	}

	if a.Flags.DbPath != "" {
		if !a.Flags.ReadFromStorage {
			// Remove existing db before re-scan
			if strings.HasSuffix(a.Flags.DbPath, ".badger") {
				os.RemoveAll(a.Flags.DbPath)
			} else {
				os.Remove(a.Flags.DbPath)
			}
		}
		if strings.HasSuffix(a.Flags.DbPath, ".badger") {
			ui.SetAnalyzer(analyze.CreateStoredAnalyzer(a.Flags.DbPath))
		} else {
			sqliteAnalyzer, err := analyze.CreateSqliteAnalyzer(a.Flags.DbPath)
			if err != nil {
				return fmt.Errorf("creating sqlite analyzer: %w", err)
			}
			ui.SetAnalyzer(sqliteAnalyzer)
		}
	}
	if a.Flags.SequentialScanning {
		ui.SetAnalyzer(analyze.CreateSeqAnalyzer())
	}
	if a.Flags.FollowSymlinks {
		ui.SetFollowSymlinks(true)
	}
	if a.Flags.ShowAnnexedSize {
		ui.SetShowAnnexedSize(true)
	}
	if a.Flags.ArchiveBrowsing {
		ui.SetArchiveBrowsing(true)
	}
	if a.Flags.CollapsePath {
		ui.SetCollapsePath(true)
	}

	// Set up time filter if any time flags are provided
	if a.Flags.Since != "" || a.Flags.Until != "" || a.Flags.MaxAge != "" || a.Flags.MinAge != "" {
		if err := a.setTimeFilters(ui); err != nil {
			return err
		}
	}
	for _, path := range paths {
		if err := a.setNoCross(path); err != nil {
			return err
		}
	}

	// Process type filters
	if len(a.Flags.TypeFilter) > 0 {
		ui.SetIncludeTypes(a.Flags.TypeFilter)
	}
	if len(a.Flags.ExcludeTypeFilter) > 0 {
		ui.SetIgnoreTypes(a.Flags.ExcludeTypeFilter)
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

	if err := a.runAction(ui, paths); err != nil {
		return err
	}

	return ui.StartUILoop()
}

func (a *App) getPaths() []string {
	if len(a.Args) > 0 {
		return a.Args
	}
	return []string{"."}
}

// resolvePaths turns the directory arguments into absolute paths, dropping
// exact duplicates.
//
// Nested paths are rejected rather than deduplicated: scanning both a directory
// and something inside it would count the nested part twice in every total gdu
// reports, and there is no answer to that which is not surprising.
func (a *App) resolvePaths() ([]string, error) {
	args := a.getPaths()
	paths := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))

	for _, arg := range args {
		path, err := filepath.Abs(arg)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	for i, path := range paths {
		for j, other := range paths {
			if i == j {
				continue
			}
			if isSubPath(other, path) {
				return nil, fmt.Errorf(
					"directory %s is nested in %s, scanning both would count it twice", other, path,
				)
			}
		}
	}

	return paths, nil
}

// checkMultiPathSupport rejects the modes that cannot represent more than one
// scanned root. Both the JSON export format and the on-disk storage are keyed
// by a single root, so there is nowhere to put a virtual top level dir.
func (a *App) checkMultiPathSupport() error {
	switch {
	case a.Flags.OutputFile != "":
		return errors.New("--output-file accepts only one directory to scan")
	case a.Flags.DbPath != "":
		return errors.New("--db accepts only one directory to scan")
	}
	return nil
}

// isSubPath reports whether path lies inside parent. Both must be absolute and
// cleaned, as returned by filepath.Abs.
func isSubPath(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." {
		return false
	}
	// only a leading ".." path *element* means "outside parent"; a name such as
	// "..foo" is an ordinary child
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (a *App) setMaxProcs() {
	if a.Flags.MaxCores < 1 || a.Flags.MaxCores > runtime.NumCPU() {
		return
	}

	runtime.GOMAXPROCS(a.Flags.MaxCores)

	// runtime.GOMAXPROCS(n) with n < 1 doesn't change current setting so we use it to check current value
	log.Printf("Max cores set to %d", runtime.GOMAXPROCS(0))
}

func (a *App) setTimeFilters(ui UI) error {
	loc := time.Local
	now := time.Now()

	timeFilter, err := timefilter.NewTimeFilter(
		a.Flags.Since,
		a.Flags.Until,
		a.Flags.MaxAge,
		a.Flags.MinAge,
		now,
		loc,
	)
	if err != nil {
		return fmt.Errorf("invalid time filter: %w", err)
	}

	if !timeFilter.IsEmpty() {
		timeFilterFunc := func(mtime time.Time) bool {
			return timeFilter.IncludeByTimeFilter(mtime, loc)
		}
		ui.SetTimeFilter(timeFilterFunc)

		// If this is a TUI, also set the filter info for display
		if tuiUI, ok := ui.(*tui.UI); ok {
			tuiUI.SetTimeFilterWithInfo(timeFilter, loc)
		}
	}
	return nil
}

func (a *App) createUI(outputAttributes gfs.JSONAttributes) (UI, error) {
	var ui UI
	var err error

	switch {
	case a.Flags.Web:
		ui = webui.CreateUI(
			a.Writer,
			a.Flags.WebConfig.Listen,
			a.Flags.WebConfig.OpenBrowser,
			a.Flags.WebConfig.Browser,
			!a.Flags.NoColor,
			a.Flags.ShowApparentSize,
			a.Flags.ShowRelativeSize,
			a.Flags.UseSIPrefix,
		)
	case a.Flags.OutputFile != "":
		var output io.Writer
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
			a.Flags.UseSIPrefix,
			a.Flags.Top,
			a.Flags.Depth,
			a.Flags.Summarize,
			outputAttributes,
		)
	case a.Flags.ShouldRunInNonInteractiveMode(a.Istty):
		fixedUnit := ""
		if a.Flags.ShowInKiB {
			fixedUnit = "k"
		}
		stdoutUI := stdout.CreateStdoutUI(
			a.Writer,
			!a.Flags.NoColor && a.Istty,
			!a.Flags.NoProgress && a.Istty,
			a.Flags.ShowApparentSize,
			a.Flags.ShowRelativeSize,
			a.Flags.Summarize,
			a.Flags.UseSIPrefix,
			a.Flags.NoPrefix,
			fixedUnit,
			a.Flags.Top,
			a.Flags.ReverseSort,
			a.Flags.Depth,
		)
		if a.Flags.NoUnicode {
			stdoutUI.UseOldProgressRunes()
		}
		if a.Flags.ShowItemCount {
			stdoutUI.SetShowItemCount()
		}
		if a.Flags.ShowSymlinkTarget {
			stdoutUI.SetShowSymlinkTarget(true)
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

// nolint:gocyclo,funlen // This function is a suite of if statements
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
	if a.Flags.Style.Marked.TextColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetMarkedTextColor(tcell.GetColor(a.Flags.Style.Marked.TextColor))
		})
	}
	if a.Flags.Style.Marked.BackgroundColor != "" {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetMarkedBackgroundColor(tcell.GetColor(a.Flags.Style.Marked.BackgroundColor))
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
	if a.Flags.Style.ShowBarPercentage {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetShowBarPercentage()
		})
	}
	if a.Flags.Style.ShowItemCountBar {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetShowItemCountBar()
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
	if a.Flags.ShowSymlinkTarget {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetShowSymlinkTarget(true)
		})
	}
	if a.Flags.NoDelete {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetNoDelete()
		})
	}
	if a.Flags.NoViewFile {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetNoViewFile()
		})
	}
	if a.Flags.NoSpawnShell {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetNoSpawnShell()
		})
	}
	if a.Flags.NoConfirmQuit {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetConfirmQuit(false)
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
	if a.Flags.BrowseParentDirs {
		opts = append(opts, func(ui *tui.UI) {
			ui.SetBrowseParentDirs()
		})
	}
	opts = append(opts, func(ui *tui.UI) {
		ui.SetShowDiskProgressBar(a.Flags.Style.ProgressModal.ShowDiskProgressBar)
	})
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

func (a *App) runAction(ui UI, paths []string) error {
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
	default:
		// Do not materialize dataless files on macOS. Without this, scanning
		// an iCloud Drive / Google Drive / OneDrive tree is orders of
		// magnitude slower. This is a no-op on other platforms.
		if err := gfs.PreventDatalessMaterialization(); err != nil {
			log.Printf("Could not prevent dataless materialization: %s", err)
		}

		scanPaths := make([]string, 0, len(paths))
		for _, path := range paths {
			if build.RootPathPrefix != "" {
				path = build.RootPathPrefix + path
			}

			if _, err := a.PathChecker(path); err != nil {
				return err
			}
			scanPaths = append(scanPaths, path)
		}

		if len(scanPaths) == 1 {
			log.Printf("Analyzing path: %s", scanPaths[0])
			if err := ui.AnalyzePath(scanPaths[0], nil); err != nil {
				return fmt.Errorf("scanning dir: %w", err)
			}
			break
		}

		log.Printf("Analyzing paths: %s", strings.Join(scanPaths, ", "))
		if err := ui.AnalyzePaths(scanPaths); err != nil {
			return fmt.Errorf("scanning dirs: %w", err)
		}
	}
	return nil
}
