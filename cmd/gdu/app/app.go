package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/dundee/gdu/v6/build"
	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/report"
	"github.com/dundee/gdu/v6/internal/stdout"
	"github.com/dundee/gdu/v6/internal/tui"
	"github.com/dundee/gdu/v6/internal/webui"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
	gfs "github.com/dundee/gdu/v6/pkg/fs"
)

// UI is what every frontend does; it is configured when it is created.
type UI interface {
	ListDevices(getter device.DevicesInfoGetter) error
	AnalyzePath(path string, parentDir gfs.Item) error
	// AnalyzePaths scans several roots and presents them under one virtual top
	// level dir. Implementations that cannot represent more than one root
	// return an error.
	AnalyzePaths(paths []string) error
	ReadAnalysis(input io.Reader) error
	// ReadFromStorage presents the analysis of path stored by an earlier scan
	// in the database of the analyzer (--db).
	ReadFromStorage(path string) error
	StartUILoop() error
}

// Flags define flags accepted by Run
type Flags struct {
	Style               Style    `yaml:"style"`
	Sorting             Sorting  `yaml:"sorting"`
	CfgFile             string   `yaml:"-"`
	LogFile             string   `yaml:"log-file"`
	InputFile           string   `yaml:"input-file"`
	OutputFile          string   `yaml:"output-file"`
	OutputAttrs         string   `yaml:"output-attrs"`
	IgnoreFromFile      string   `yaml:"ignore-from-file"`
	IgnoreFromGitignore string   `yaml:"ignore-from-gitignore"`
	IgnoreDirs          []string `yaml:"ignore-dirs"`
	IgnoreDirPatterns   []string `yaml:"ignore-dir-patterns"`
	TypeFilter          []string `yaml:"type"`
	ExcludeTypeFilter   []string `yaml:"exclude-type"`
	// MaxCores bounds the processors gdu uses; the gdu command sets it for
	// the process, Run does not
	MaxCores           int       `yaml:"max-cores"`
	Top                int       `yaml:"top"`
	Depth              int       `yaml:"depth"`
	SequentialScanning bool      `yaml:"sequential-scanning"`
	ShowDisks          bool      `yaml:"-"`
	ShowApparentSize   bool      `yaml:"show-apparent-size"`
	ShowRelativeSize   bool      `yaml:"show-relative-size"`
	ShowAnnexedSize    bool      `yaml:"show-annexed-size"`
	ExclusiveSize      bool      `yaml:"exclusive-size"`
	ShowVersion        bool      `yaml:"-"`
	SelfUpdate         bool      `yaml:"-"`
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
	TrashCommand       string    `yaml:"trash-command"`
	Shell              string    `yaml:"shell"`
	Since              string    `yaml:"since"`
	Until              string    `yaml:"until"`
	MaxAge             string    `yaml:"max-age"`
	MinAge             string    `yaml:"min-age"`
	ArchiveBrowsing    bool      `yaml:"archive-browsing"`
	CollapsePath       bool      `yaml:"collapse-path"`
	ShowSymlinkTarget  bool      `yaml:"show-symlink-target"`
	CtrlCQuits         bool      `yaml:"ctrl-c-quits"`
	BrowseParentDirs   bool      `yaml:"browse-parent-dirs"`
	Web                bool      `yaml:"-"`
	WebConfig          WebConfig `yaml:"web"`
	// Cleanup holds user rules and exclusions for the web UI's cleanup
	// projections.
	Cleanup CleanupConfig `yaml:"cleanup"`
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

// TermApplication is the terminal application gdu runs its interactive UI in,
// usually a *tview.Application.
type TermApplication interface {
	Run() error
	Stop()
	Suspend(f func()) bool
	SetRoot(root tview.Primitive, fullscreen bool) *tview.Application
	SetFocus(p tview.Primitive) *tview.Application
	SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) *tview.Application
	SetMouseCapture(
		capture func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction),
	) *tview.Application
	QueueUpdateDraw(f func()) *tview.Application
	SetBeforeDrawFunc(func(screen tcell.Screen) bool) *tview.Application
}

// App defines the main application
type App struct {
	Writer io.Writer
	// ErrWriter is told what went wrong without stopping gdu, such as the
	// directories a scan could not read; os.Stderr when nil
	ErrWriter   io.Writer
	TermApp     TermApplication
	Screen      tcell.Screen
	Getter      device.DevicesInfoGetter
	Flags       *Flags
	PathChecker func(string) (fs.FileInfo, error)
	Args        []string
	Istty       bool
}

// Run starts gdu main logic
func (a *App) Run() (err error) {
	if a.Flags.ShowVersion {
		fmt.Fprintln(a.Writer, "Version:\t", build.Version)
		fmt.Fprintln(a.Writer, "Built time:\t", build.Time)
		fmt.Fprintln(a.Writer, "Built user:\t", build.User)
		return nil
	}

	slog.Info("runtime flags", "flags", *a.Flags)

	paths, err := a.resolvePaths()
	if err != nil {
		return err
	}
	if err := a.validate(paths); err != nil {
		return err
	}
	settings, err := a.settings(paths)
	if err != nil {
		return err
	}
	slog.Info("settings", "settings", settings)

	front := a.frontend()
	analyzer, err := a.analyzer(front, &settings)
	if err != nil {
		return err
	}
	if db, ok := analyzer.(analyze.Database); ok {
		// a database stays open for the run, and closing it writes out what
		// it holds in memory
		defer func() {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("closing the database: %w", closeErr))
			}
		}()
	}
	ui, err := a.createUI(front, analyzer, &settings)
	if err != nil {
		return err
	}

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

// createUI returns the frontend front for the settings s, scanning with
// analyzer.
func (a *App) createUI(front frontend, analyzer analyze.Analyzer, s *config.Settings) (UI, error) {
	switch front {
	case frontendWeb:
		return webui.CreateUI(a.Writer, analyzer, s)
	case frontendExport:
		var output io.Writer = os.Stdout
		if a.Flags.OutputFile != "-" {
			file, err := newOutputFile(a.Flags.OutputFile)
			if err != nil {
				return nil, err
			}
			output = file
		}
		return report.CreateExportUI(a.Writer, output, analyzer, s), nil
	case frontendStdout:
		errWriter := a.ErrWriter
		if errWriter == nil {
			errWriter = os.Stderr
		}
		return stdout.CreateStdoutUI(a.Writer, errWriter, analyzer, s), nil
	case frontendTUI:
	}
	return tui.CreateUI(a.TermApp, a.Screen, os.Stdout, analyzer, s), nil
}

func (a *App) runAction(ui UI, paths []string) error {
	if a.Flags.Profiling {
		// a mux of its own, rather than the process's http.DefaultServeMux
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		go func() {
			slog.Error("profiling server", "err", http.ListenAndServe("localhost:6060", mux))
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
		// a database holds a single scanned root, see checkMultiPathSupport;
		// the path is not checked, the analysis is readable even when the
		// directory is gone
		path := build.RootPathPrefix + paths[0]
		slog.Info("reading stored analysis", "path", path)
		if err := ui.ReadFromStorage(path); err != nil {
			return fmt.Errorf("reading stored analysis: %w", err)
		}
	default:
		// Do not materialize dataless files on macOS. Without this, scanning
		// an iCloud Drive / Google Drive / OneDrive tree is orders of
		// magnitude slower. This is a no-op on other platforms.
		if err := gfs.PreventDatalessMaterialization(); err != nil {
			slog.Warn("preventing dataless materialization", "err", err)
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
			slog.Info("analyzing", "path", scanPaths[0])
			if err := ui.AnalyzePath(scanPaths[0], nil); err != nil {
				return fmt.Errorf("scanning dir: %w", err)
			}
			break
		}

		slog.Info("analyzing", "paths", scanPaths)
		if err := ui.AnalyzePaths(scanPaths); err != nil {
			return fmt.Errorf("scanning dirs: %w", err)
		}
	}
	return nil
}
