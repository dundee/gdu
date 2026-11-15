package stdout

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dundee/gdu/v6/internal/common"
	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/session"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/fatih/color"
)

// UI struct
type UI struct {
	output io.Writer
	// errOutput is told what a scan could not read
	errOutput io.Writer
	*common.UI
	session           *session.Session
	red               *color.Color
	orange            *color.Color
	blue              *color.Color
	cyan              *color.Color
	progressRunes     []rune
	showItemCnt       bool
	showSymlinkTarget bool
	top               int
	depth             int
	summarize         bool
	noPrefix          bool
	fixedBase         float64
	fixedSuffix       string
	reverseSort       bool
	configuredSortBy  string
	configuredOrder   string
}

// CreateStdoutUI returns the UI printing the analysis to output, and what it
// could not read to errOutput, as du does.
func CreateStdoutUI(output, errOutput io.Writer, analyzer analyze.Analyzer, s *config.Settings) *UI {
	ui := &UI{
		UI:                common.NewUI(analyzer, s),
		output:            output,
		errOutput:         errOutput,
		summarize:         s.Display.Summarize,
		noPrefix:          s.Display.NoPrefix,
		top:               s.Display.Top,
		reverseSort:       s.Display.ReverseSort,
		depth:             s.Display.Depth,
		showItemCnt:       s.Display.ItemCount,
		showSymlinkTarget: s.Display.SymlinkTarget,
		configuredSortBy:  s.Display.SortBy,
		configuredOrder:   s.Display.SortOrder,
	}
	if s.Display.FixedUnit != "" {
		ui.setFixedUnit(s.Display.FixedUnit)
	} else if !s.Display.NoPrefix && !s.Display.SIPrefix {
		ui.SetBlockSizeFromEnvironment()
	}
	ui.progressRunes = common.SpinnerRunes(s.Display.NoUnicode)
	ui.red = common.BoldColor(color.FgRed, s.Display.Colors)
	ui.orange = common.BoldColor(color.FgYellow, s.Display.Colors)
	ui.blue = common.BoldColor(color.FgBlue, s.Display.Colors)
	ui.cyan = common.BoldColor(color.FgCyan, s.Display.Colors)
	ui.session = session.New(ui.UI)

	return ui
}

// setFixedUnit shows every size in the unit unitChar: "k", "m" or "g".
func (ui *UI) setFixedUnit(unitChar string) {
	k, m, g := common.Ki, common.Mi, common.Gi
	suffixMap := map[string]string{"k": " KiB", "m": " MiB", "g": " GiB"}

	if ui.UseSIPrefix {
		k, m, g = common.K, common.M, common.G
		suffixMap = map[string]string{"k": " kB", "m": " MB", "g": " GB"}
	}

	switch unitChar {
	case "k":
		ui.fixedBase = k
		ui.fixedSuffix = suffixMap["k"]
	case "m":
		ui.fixedBase = m
		ui.fixedSuffix = suffixMap["m"]
	case "g":
		ui.fixedBase = g
		ui.fixedSuffix = suffixMap["g"]
	}
}

// StartUILoop stub
func (ui *UI) StartUILoop() error {
	return nil
}

// ListDevices lists mounted devices and shows their disk usage
func (ui *UI) ListDevices(getter device.DevicesInfoGetter) error {
	devices, err := getter.GetDevicesInfo()
	if err != nil {
		return err
	}

	maxDeviceNameLength := maxInt(maxLength(
		devices,
		func(device *device.Device) string { return device.Name },
	), len("Devices"))

	var sizeLength, percentLength int
	if ui.UseColors {
		sizeLength = 20
		percentLength = 16
	} else {
		sizeLength = 9
		percentLength = 5
	}

	lineFormat := fmt.Sprintf(
		"%%%ds %%%ds %%%ds %%%ds %%%ds %%s\n",
		maxDeviceNameLength,
		sizeLength,
		sizeLength,
		sizeLength,
		percentLength,
	)

	fmt.Fprintf(
		ui.output,
		fmt.Sprintf("%%%ds %%9s %%9s %%9s %%5s %%s\n", maxDeviceNameLength),
		"Device",
		"Size",
		"Used",
		"Free",
		"Used%",
		"Mount point",
	)

	for _, device := range devices {
		usedPercent := math.Round(float64(device.Size-device.Free) / float64(device.Size) * 100)

		fmt.Fprintf(
			ui.output,
			lineFormat,
			device.Name,
			ui.formatSize(device.Size),
			ui.formatSize(device.Size-device.Free),
			ui.formatSize(device.Free),
			ui.red.Sprintf("%.f%%", usedPercent),
			device.MountPoint)
	}

	return nil
}

// measureSharedUsage measures a file given as the path to analyze.
var measureSharedUsage = analyze.MeasureSharedUsage

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, _ fs.Item) error {
	// When path is a regular file, create a File item directly so that
	// apparent-size (GetSize) and disk-usage (GetUsage) are both correct.
	// Running a file through AnalyzeDir + UpdateStats would report the
	// default 4096-byte directory overhead instead of the real values.
	info, err := os.Stat(path)
	if err == nil && info.Mode().IsRegular() {
		file := analyze.CreateFileItem(path, info)
		if ui.ScanOptions().ExclusiveSize {
			file.Usage, file.SharedUsage = measureSharedUsage(path, info, file.Usage)
		}
		ui.printTotalItem(file)
		return nil
	}

	dir := ui.session.Scan(context.Background(), []string{path}, ui.scanObserver())
	ui.clearProgress()
	ui.reportReadErrors()
	ui.printAnalyzedDir(dir)

	return nil
}

// AnalyzePaths analyzes several paths and reports them together, grouped under
// a virtual top level directory.
func (ui *UI) AnalyzePaths(paths []string) error {
	if len(paths) == 1 {
		return ui.AnalyzePath(paths[0], nil)
	}

	// The top dir analyzer returns SimpleDir, a reduced item that cannot be
	// re-parented, so it cannot live under a virtual root. Trade its lower
	// memory use for the full tree, as --top and --depth already do.
	if _, ok := ui.Analyzer.(*analyze.TopDirAnalyzer); ok {
		ui.Analyzer = analyze.CreateAnalyzer()
	}

	dir := ui.session.Scan(context.Background(), paths, ui.scanObserver())
	ui.clearProgress()
	ui.reportReadErrors()
	ui.printAnalyzedDir(dir)

	return nil
}

// reportReadErrors tells errOutput what the scan could not read, one line
// each, as du does; the listing on output stays as it is.
func (ui *UI) reportReadErrors() {
	for _, e := range ui.session.ReadErrors() {
		fmt.Fprintln(ui.errOutput, readErrorMessage(e))
	}
}

// readErrorMessage tells e in the words of GNU du.
func readErrorMessage(e analyze.ReadError) string {
	action := "cannot access"
	if e.Dir {
		action = "cannot read directory"
	}
	return fmt.Sprintf("gdu: %s '%s': %s", action, e.Path, errorReason(e.Err))
}

// errorReason returns what went wrong, as the system says it, without the
// operation and path err repeats: "Permission denied".
func errorReason(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	reason := err.Error()
	first, size := utf8.DecodeRuneInString(reason)
	return string(unicode.ToUpper(first)) + reason[size:]
}

func (ui *UI) printAnalyzedDir(dir fs.Item) {
	switch {
	case ui.top > 0:
		ui.printTopFiles(dir)
	case ui.depth > 0:
		ui.printDirWithDepth(dir, 0)
	case ui.summarize:
		ui.printTotalItem(dir)
	default:
		ui.showDir(dir)
	}
}

// ReadFromStorage prints the analysis of path stored by an earlier scan with
// --db.
func (ui *UI) ReadFromStorage(path string) error {
	dir, err := ui.session.Open(path)
	if err != nil {
		return err
	}
	ui.printAnalyzedDir(dir)
	return nil
}

// sortSettings honors the config, the displayed size metric and --reverse-sort.
func (ui *UI) sortSettings() (fs.SortBy, fs.SortOrder) {
	sortOrder := fs.ParseSortOrder(ui.configuredOrder)
	if ui.reverseSort {
		if sortOrder == fs.SortAsc {
			sortOrder = fs.SortDesc
		} else {
			sortOrder = fs.SortAsc
		}
	}

	sortBy := fs.ParseSortBy(ui.configuredSortBy)
	if sortBy == fs.SortBySize && ui.ShowApparentSize {
		sortBy = fs.SortByApparentSize
	}

	return sortBy, sortOrder
}

func (ui *UI) showDir(dir fs.Item) {
	sort, sortOrder := ui.sortSettings()

	for file := range dir.GetFiles(sort, sortOrder) {
		ui.printItem(dir, file)
	}
}

// printTopFiles ranks by the displayed size metric regardless of the
// configured sorting, because --top selects the largest files.
func (ui *UI) printTopFiles(file fs.Item) {
	sortBy := fs.SortBySize
	if ui.ShowApparentSize {
		sortBy = fs.SortByApparentSize
	}
	collected := analyze.CollectTopFiles(file, ui.top, sortBy)
	for _, file := range collected {
		ui.printItemPath(file)
	}
}

func (ui *UI) printTotalItem(file fs.Item) {
	var lineFormat string
	if ui.UseColors {
		lineFormat = "%20s %s\n"
	} else {
		lineFormat = "%9s %s\n"
	}

	var size int64
	if ui.ShowApparentSize {
		size = file.GetSize()
	} else {
		size = file.GetUsage()
	}

	fmt.Fprintf(
		ui.output,
		lineFormat,
		ui.formatSize(size),
		file.GetName(),
	)
}

// printItem prints one row of a directory listing. parent is the directory
// being listed, which decides how the item is labelled; it may be nil.
func (ui *UI) printItem(parent, file fs.Item) {
	var lineFormat string
	if ui.showItemCnt {
		if ui.UseColors {
			lineFormat = "%s %23s %25s %s\n"
		} else {
			lineFormat = "%s %9s %11s %s\n"
		}
	} else {
		if ui.UseColors {
			lineFormat = "%s %23s %s\n"
		} else {
			lineFormat = "%s %9s %s\n"
		}
	}

	var size int64
	if ui.ShowApparentSize {
		size = file.GetSize()
	} else {
		size = file.GetUsage()
	}

	name := analyze.ItemDisplayName(parent, file)
	if file.IsDir() {
		// a scanned root is labelled with its absolute path, which already
		// reads as a path and must not gain another leading separator
		if !analyze.IsVirtualRootDir(parent) {
			name = "/" + name
		}
		name = ui.blue.Sprint(name)
	}

	// Append symlink target with cyan name (like ls --color)
	if ui.showSymlinkTarget {
		if si, ok := file.(fs.SymlinkItem); ok {
			if target := si.GetSymlinkTarget(); target != "" {
				name = ui.cyan.Sprint(file.GetName()) + " -> " + target
			}
		}
	}

	if ui.showItemCnt {
		fmt.Fprintf(
			ui.output,
			lineFormat,
			string(file.GetFlag()),
			ui.formatSize(size),
			ui.formatCount(fs.DisplayedItemCount(file)),
			name,
		)
		return
	}

	fmt.Fprintf(
		ui.output,
		lineFormat,
		string(file.GetFlag()),
		ui.formatSize(size),
		name,
	)
}

func (ui *UI) printItemPath(file fs.Item) {
	var lineFormat string
	if ui.UseColors {
		lineFormat = "%20s %s\n"
	} else {
		lineFormat = "%9s %s\n"
	}

	var size int64
	if ui.ShowApparentSize {
		size = file.GetSize()
	} else {
		size = file.GetUsage()
	}

	if file.IsDir() {
		fmt.Fprintf(ui.output,
			lineFormat,
			ui.formatSize(size),
			ui.blue.Sprint(file.GetPath()))
	} else {
		fmt.Fprintf(ui.output,
			lineFormat,
			ui.formatSize(size),
			file.GetPath())
	}
}

func (ui *UI) printDirWithDepth(dir fs.Item, currentDepth int) {
	// Print current directory
	ui.printItemPath(dir)

	// If we haven't reached the max depth, print contents
	if currentDepth < ui.depth && dir.IsDir() {
		files := dir.GetFiles(ui.sortSettings())

		// Print all files at this depth level
		for file := range files {
			if file.IsDir() {
				// Recurse into subdirectories
				ui.printDirWithDepth(file, currentDepth+1)
			} else {
				// Print regular files
				ui.printItemPath(file)
			}
		}
	}
}

// ReadAnalysis reads analysis report from JSON file
func (ui *UI) ReadAnalysis(input io.Reader) error {
	stop := ui.showReadingProgress()
	dir, err := ui.session.Load(input)
	stop()
	if err != nil {
		return err
	}

	ui.printAnalyzedDir(dir)

	return nil
}

// showReadingProgress spins until the returned function is called.
func (ui *UI) showReadingProgress() (stop func()) {
	if !ui.ShowProgress {
		return func() {}
	}
	quit := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; ; i = (i + 1) % len(ui.progressRunes) {
			fmt.Fprint(ui.output, emptyRow(40))
			fmt.Fprintf(ui.output, "\r %s Reading analysis from file...", string(ui.progressRunes[i]))
			select {
			case <-quit:
				fmt.Fprint(ui.output, emptyRow(40), "\r")
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		close(quit)
		<-stopped
	}
}

// scanObserver returns the observer printing the progress of a scan, or nil
// when no progress is shown.
func (ui *UI) scanObserver() session.Observer {
	if !ui.ShowProgress {
		return nil
	}
	i := 0
	return func(p session.Progress) {
		fmt.Fprint(ui.output, emptyRow(100))
		fmt.Fprintf(ui.output, "\r %s ", string(ui.progressRunes[i]))
		if p.Phase == session.Finalizing {
			fmt.Fprint(ui.output, "Calculating disk usage...")
		} else {
			fmt.Fprint(ui.output, "Scanning... Total items: "+
				ui.red.Sprint(common.FormatNumber(p.ItemCount))+
				" size: "+
				ui.formatSize(p.TotalUsage))
		}
		i = (i + 1) % len(ui.progressRunes)
	}
}

// clearProgress removes the progress line printed by scanObserver.
func (ui *UI) clearProgress() {
	if ui.ShowProgress {
		fmt.Fprint(ui.output, emptyRow(100), "\r")
	}
}

// emptyRow returns a carriage return followed by width spaces, which blanks
// a progress line.
func emptyRow(width int) string {
	return "\r" + strings.Repeat(" ", width)
}

func (ui *UI) formatCount(count int64) string {
	count64 := float64(count)

	switch {
	case count64 >= common.G:
		return ui.red.Sprintf("%.1f", float64(count)/float64(common.G)) + "G"
	case count64 >= common.M:
		return ui.red.Sprintf("%.1f", float64(count)/float64(common.M)) + "M"
	case count64 >= common.K:
		return ui.red.Sprintf("%.1f", float64(count)/float64(common.K)) + "k"
	default:
		return ui.red.Sprintf("%d", count)
	}
}

func (ui *UI) formatSize(size int64) string {
	if ui.noPrefix {
		return ui.orange.Sprintf("%d", size)
	}
	if formatted, ok := ui.FormatBlockSize(size); ok {
		return ui.orange.Sprint(formatted)
	}
	if ui.fixedBase > 0 {
		val := float64(size) / ui.fixedBase
		return ui.orange.Sprintf("%.1f", val) + ui.fixedSuffix
	}
	if ui.UseSIPrefix {
		return ui.formatWithDecPrefix(size)
	}
	return ui.formatWithBinPrefix(size)
}

func (ui *UI) formatWithBinPrefix(size int64) string {
	fsize := float64(size)
	asize := math.Abs(fsize)

	switch {
	case asize >= common.Ei:
		return ui.orange.Sprintf("%.1f", fsize/common.Ei) + " EiB"
	case asize >= common.Pi:
		return ui.orange.Sprintf("%.1f", fsize/common.Pi) + " PiB"
	case asize >= common.Ti:
		return ui.orange.Sprintf("%.1f", fsize/common.Ti) + " TiB"
	case asize >= common.Gi:
		return ui.orange.Sprintf("%.1f", fsize/common.Gi) + " GiB"
	case asize >= common.Mi:
		return ui.orange.Sprintf("%.1f", fsize/common.Mi) + " MiB"
	case asize >= common.Ki:
		return ui.orange.Sprintf("%.1f", fsize/common.Ki) + " KiB"
	default:
		return ui.orange.Sprintf("%d", size) + " B"
	}
}

func (ui *UI) formatWithDecPrefix(size int64) string {
	fsize := float64(size)
	asize := math.Abs(fsize)

	switch {
	case asize >= common.E:
		return ui.orange.Sprintf("%.1f", fsize/common.E) + " EB"
	case asize >= common.P:
		return ui.orange.Sprintf("%.1f", fsize/common.P) + " PB"
	case asize >= common.T:
		return ui.orange.Sprintf("%.1f", fsize/common.T) + " TB"
	case asize >= common.G:
		return ui.orange.Sprintf("%.1f", fsize/common.G) + " GB"
	case asize >= common.M:
		return ui.orange.Sprintf("%.1f", fsize/common.M) + " MB"
	case asize >= common.K:
		return ui.orange.Sprintf("%.1f", fsize/common.K) + " kB"
	default:
		return ui.orange.Sprintf("%d", size) + " B"
	}
}

func maxLength(list []*device.Device, keyGetter func(*device.Device) string) int {
	maxLen := 0
	var s string
	for _, item := range list {
		s = keyGetter(item)
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	return maxLen
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}
