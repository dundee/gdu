package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/dundee/gdu/v6/internal/common"
	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/timefilter"
	"github.com/dundee/gdu/v6/pkg/device"
)

// frontend is the kind of UI a run presents its result in.
type frontend int

const (
	frontendTUI frontend = iota
	frontendStdout
	frontendExport
	frontendWeb
)

// frontend returns the UI the flags ask for.
func (a *App) frontend() frontend {
	switch {
	case a.Flags.Web:
		return frontendWeb
	case a.Flags.OutputFile != "":
		return frontendExport
	case a.Flags.ShouldRunInNonInteractiveMode(a.Istty):
		return frontendStdout
	}
	return frontendTUI
}

// names of the flags which conflict with several others
const (
	flagOutputFile      = "--output-file"
	flagShowDisks       = "--show-disks"
	flagInputFile       = "--input-file"
	flagReadFromStorage = "--read-from-storage"
	flagWeb             = "--web"
)

// conflict is a pair of flags which cannot be used at once.
type conflict struct {
	first, second string
	set           func(f *Flags) bool
}

// conflicts are the flags which contradict each other. gdu refuses them
// rather than silently preferring one.
var conflicts = []conflict{
	{"--no-prefix", "--si", func(f *Flags) bool { return f.NoPrefix && f.UseSIPrefix }},
	{"--interactive", "--non-interactive", func(f *Flags) bool { return f.Interactive && f.NonInteractive }},
	{"--sequential", "--db", func(f *Flags) bool { return f.SequentialScanning && f.DbPath != "" }},
	{flagWeb, flagOutputFile, func(f *Flags) bool { return f.Web && f.OutputFile != "" }},
	{flagWeb, flagShowDisks, func(f *Flags) bool { return f.Web && f.ShowDisks }},
	{flagInputFile, flagReadFromStorage, func(f *Flags) bool { return f.InputFile != "" && f.ReadFromStorage }},
	{flagShowDisks, flagInputFile, func(f *Flags) bool { return f.ShowDisks && f.InputFile != "" }},
	{flagShowDisks, flagReadFromStorage, func(f *Flags) bool { return f.ShowDisks && f.ReadFromStorage }},
	{flagOutputFile, flagShowDisks, func(f *Flags) bool { return f.OutputFile != "" && f.ShowDisks }},
	{flagOutputFile, flagInputFile, func(f *Flags) bool { return f.OutputFile != "" && f.InputFile != "" }},
}

// validate rejects flags which contradict each other or lack another one,
// before anything is set up. paths are the scanned roots.
func (a *App) validate(paths []string) error {
	for _, c := range conflicts {
		if c.set(a.Flags) {
			return fmt.Errorf("%s and %s cannot be used at once", c.first, c.second)
		}
	}
	if a.Flags.OutputAttrs != "" && a.Flags.OutputFile == "" {
		return errors.New("--output-attrs requires --output-file")
	}
	if err := a.checkReadFromStorage(); err != nil {
		return err
	}
	if len(paths) > 1 {
		return a.checkMultiPathSupport()
	}
	return nil
}

// settings resolves the flags into the settings of the run, for the scanned
// roots paths. The flags are left as they are.
func (a *App) settings(paths []string) (config.Settings, error) {
	f := a.Flags
	front := a.frontend()

	attributes, err := parseJSONAttributes(f.OutputAttrs)
	if err != nil {
		return config.Settings{}, err
	}
	scan, err := a.scanSettings(paths)
	if err != nil {
		return config.Settings{}, err
	}

	// the terminal and web UIs show colours whether or not the output is a
	// terminal; stdout and the export only on one
	colors := !f.NoColor
	progress := false
	if front == frontendStdout || front == frontendExport {
		colors = colors && a.Istty
		progress = !f.NoProgress && a.Istty
	}
	fixedUnit := ""
	if f.ShowInKiB {
		fixedUnit = "k"
	}

	return config.Settings{
		Scan: scan,
		Display: config.Display{
			Colors:    colors,
			Progress:  progress,
			NoUnicode: f.NoUnicode,
			// disk usage is not available on Plan9
			ApparentSize:  f.ShowApparentSize || runtime.GOOS == "plan9",
			RelativeSize:  f.ShowRelativeSize,
			SIPrefix:      f.UseSIPrefix,
			NoPrefix:      f.NoPrefix,
			FixedUnit:     fixedUnit,
			ItemCount:     f.ShowItemCount,
			MTime:         f.ShowMTime,
			SymlinkTarget: f.ShowSymlinkTarget,
			CollapsePath:  f.CollapsePath,
			SortBy:        f.Sorting.By,
			SortOrder:     f.Sorting.Order,
			ReverseSort:   f.ReverseSort,
			Top:           f.Top,
			Depth:         f.Depth,
			Summarize:     f.Summarize,
		},
		Delete: config.Delete{
			Disabled:     f.NoDelete,
			InBackground: f.DeleteInBackground,
			InParallel:   f.DeleteInParallel,
			TrashCommand: f.TrashCommand,
		},
		TUI: config.TUI{
			Style:            styleSettings(f),
			Shell:            f.Shell,
			NoViewFile:       f.NoViewFile,
			NoSpawnShell:     f.NoSpawnShell,
			NoConfirmQuit:    f.NoConfirmQuit,
			CtrlCQuits:       f.CtrlCQuits,
			ChangeCwd:        f.ChangeCwd,
			BrowseParentDirs: f.BrowseParentDirs,
		},
		Export: config.Export{Attributes: attributes},
		Web: config.Web{
			Listen:      f.WebConfig.Listen,
			OpenBrowser: f.WebConfig.OpenBrowser,
			Browser:     f.WebConfig.Browser,
			Cleanup:     f.Cleanup.config(),
		},
	}, nil
}

// scanSettings resolves what a scan of paths leaves out and how it counts.
func (a *App) scanSettings(paths []string) (config.Scan, error) {
	f := a.Flags
	scan := config.Scan{
		IgnoreDirs:             append([]string(nil), f.IgnoreDirs...),
		IgnoreHidden:           f.NoHidden,
		IgnoreOtherFilesystems: f.NoCross,
		IncludeTypes:           f.TypeFilter,
		ExcludeTypes:           f.ExcludeTypeFilter,
		FollowSymlinks:         f.FollowSymlinks,
		AnnexedSize:            f.ShowAnnexedSize,
		ExclusiveSize:          f.ExclusiveSize,
		ArchiveBrowsing:        f.ArchiveBrowsing,
	}

	if f.NoCross {
		mounts, err := a.Getter.GetMounts()
		if err != nil {
			return scan, fmt.Errorf("loading mount points: %w", err)
		}
		for _, path := range paths {
			nested := device.GetNestedMountpointsPaths(path, mounts)
			slog.Info("ignoring mount points", "paths", nested)
			scan.IgnoreDirs = append(scan.IgnoreDirs, nested...)
		}
	}

	// the scanned roots anchor the patterns of a .gitignore starting with /
	pattern, err := common.IgnorePattern(f.IgnoreDirPatterns, f.IgnoreFromFile, f.IgnoreFromGitignore, paths)
	if err != nil {
		return scan, err
	}
	scan.IgnoreDirPattern = pattern

	if f.Since != "" || f.Until != "" || f.MaxAge != "" || f.MinAge != "" {
		loc := time.Local
		filter, err := timefilter.NewTimeFilter(f.Since, f.Until, f.MaxAge, f.MinAge, time.Now(), loc)
		if err != nil {
			return scan, fmt.Errorf("invalid time filter: %w", err)
		}
		if !filter.IsEmpty() {
			scan.TimeFilter, scan.TimeLocation = filter, loc
		}
	}
	return scan, nil
}

// styleSettings resolves the looks of the terminal UI.
func styleSettings(f *Flags) config.Style {
	s := f.Style
	return config.Style{
		SelectedRowText:       s.SelectedRow.TextColor,
		SelectedRowBackground: s.SelectedRow.BackgroundColor,
		MarkedText:            s.Marked.TextColor,
		MarkedBackground:      s.Marked.BackgroundColor,
		FooterText:            s.Footer.TextColor,
		FooterBackground:      s.Footer.BackgroundColor,
		FooterNumber:          s.Footer.NumberColor,
		HeaderText:            s.Header.TextColor,
		HeaderBackground:      s.Header.BackgroundColor,
		HeaderHidden:          s.Header.Hidden,
		ResultRowNumber:       s.ResultRow.NumberColor,
		ResultRowDirectory:    s.ResultRow.DirectoryColor,
		CurrentItemNameMaxLen: s.ProgressModal.CurrentItemNameMaxLen,
		ShowDiskProgressBar:   s.ProgressModal.ShowDiskProgressBar,
		OldSizeBar:            s.UseOldSizeBar || f.NoUnicode,
		ShowBarPercentage:     s.ShowBarPercentage,
		ShowItemCountBar:      s.ShowItemCountBar,
	}
}

// checkReadFromStorage rejects --read-from-storage without a database to read
// from, rather than letting the analyzer create an empty one.
func (a *App) checkReadFromStorage() error {
	if !a.Flags.ReadFromStorage {
		return nil
	}
	if a.Flags.DbPath == "" {
		return errors.New("--read-from-storage requires --db")
	}
	if _, err := os.Stat(a.Flags.DbPath); err != nil {
		return fmt.Errorf("reading the database: %w", err)
	}
	return nil
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
