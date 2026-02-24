package stdout

import (
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/report"
	"github.com/fatih/color"
)

// UI struct
type UI struct {
	output io.Writer
	*common.UI
	red         *color.Color
	orange      *color.Color
	blue        *color.Color
	showItemCnt bool
	top         int
	depth       int
	summarize   bool
	noPrefix    bool
	fixedBase   float64
	fixedSuffix string
	reverseSort bool
}

var (
	progressRunes      = []rune(`⠇⠏⠋⠙⠹⠸⠼⠴⠦⠧`)
	progressRunesOld   = []rune(`-\\|/`)
	progressRunesCount = len(progressRunes)
)

// CreateStdoutUI creates UI for stdout
func CreateStdoutUI(
	output io.Writer,
	useColors bool,
	showProgress bool,
	showApparentSize bool,
	showRelativeSize bool,
	summarize bool,
	useSIPrefix bool,
	noPrefix bool,
	fixedUnit string,
	top int,
	reverseSort bool,
	depth int,
) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowProgress:     showProgress,
			ShowApparentSize: showApparentSize,
			ShowRelativeSize: showRelativeSize,
			Analyzer:         analyze.CreateAnalyzer(),
			UseSIPrefix:      useSIPrefix,
		},
		output:      output,
		summarize:   summarize,
		noPrefix:    noPrefix,
		top:         top,
		reverseSort: reverseSort,
		depth:       depth,
	}
	if fixedUnit != "" {
		ui.SetFixedUnit(fixedUnit)
	}
	ui.red = color.New(color.FgRed).Add(color.Bold)
	ui.orange = color.New(color.FgYellow).Add(color.Bold)
	ui.blue = color.New(color.FgBlue).Add(color.Bold)

	if !useColors {
		color.NoColor = true
	}

	return ui
}
func (ui *UI) SetFixedUnit(unitChar string) {
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

func (ui *UI) SetShowItemCount() {
	ui.showItemCnt = true
}

func (ui *UI) UseOldProgressRunes() {
	progressRunes = progressRunesOld
	progressRunesCount = len(progressRunes)
}

// StartUILoop stub
func (ui *UI) StartUILoop() error {
	return nil
}

// SetCollapsePath sets the flag to collapse paths
func (ui *UI) SetCollapsePath(value bool) {
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

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, _ fs.Item) error {
	var (
		dir             fs.Item
		wait            sync.WaitGroup
		updateStatsDone chan struct{}
	)
	updateStatsDone = make(chan struct{}, 1)

	if ui.ShowProgress {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ui.updateProgress(updateStatsDone)
		}()
	}

	wait.Add(1)
	go func() {
		defer wait.Done()
		dir = ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter())
		dir.UpdateStats(make(fs.HardLinkedItems, 10))
		updateStatsDone <- struct{}{}
	}()

	wait.Wait()

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

	return nil
}

// ReadFromStorage reads analysis data from persistent key-value storage
func (ui *UI) ReadFromStorage(storagePath, path string) error {
	storage := analyze.NewStorage(storagePath, path)
	closeFn := storage.Open()
	defer closeFn()

	dir, err := storage.GetDirForPath(path)
	if err != nil {
		return err
	}

	switch {
	case ui.top > 0:
		ui.printTopFiles(dir)
	case ui.summarize:
		ui.printTotalItem(dir)
	default:
		ui.showDir(dir)
	}
	return nil
}

func (ui *UI) showDir(dir fs.Item) {
	sortOrder := fs.SortDesc
	if ui.reverseSort {
		sortOrder = fs.SortAsc
	}

	for file := range dir.GetFiles(fs.SortBySize, sortOrder) {
		ui.printItem(file)
	}
}

func (ui *UI) printTopFiles(file fs.Item) {
	collected := analyze.CollectTopFiles(file, ui.top)
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

func (ui *UI) printItem(file fs.Item) {
	var lineFormat string
	if ui.showItemCnt {
		if ui.UseColors {
			lineFormat = "%s %20s %11s %s\n"
		} else {
			lineFormat = "%s %9s %11s %s\n"
		}
	} else {
		if ui.UseColors {
			lineFormat = "%s %20s %s\n"
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

	countToDisplay := file.GetItemCount()
	if file.IsDir() {
		countToDisplay--
	}

	name := file.GetName()
	if file.IsDir() {
		name = ui.blue.Sprint("/" + file.GetName())
	}

	if ui.showItemCnt {
		fmt.Fprintf(
			ui.output,
			lineFormat,
			string(file.GetFlag()),
			ui.formatSize(size),
			formatCount(countToDisplay),
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

func formatCount(count int) string {
	count64 := float64(count)

	switch {
	case count64 >= common.G:
		return fmt.Sprintf("%.1fG", float64(count)/float64(common.G))
	case count64 >= common.M:
		return fmt.Sprintf("%.1fM", float64(count)/float64(common.M))
	case count64 >= common.K:
		return fmt.Sprintf("%.1fk", float64(count)/float64(common.K))
	default:
		return fmt.Sprintf("%d", count)
	}
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
		sortOrder := fs.SortDesc
		if ui.reverseSort {
			sortOrder = fs.SortAsc
		}

		files := dir.GetFiles(fs.SortBySize, sortOrder)

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
	var (
		dir      fs.Item
		wait     sync.WaitGroup
		err      error
		doneChan chan struct{}
	)

	if ui.ShowProgress {
		wait.Add(1)
		doneChan = make(chan struct{})
		go func() {
			defer wait.Done()
			ui.showReadingProgress(doneChan)
		}()
	}

	wait.Add(1)
	go func() {
		defer wait.Done()
		dir, err = report.ReadAnalysis(input)
		if err != nil {
			if ui.ShowProgress {
				doneChan <- struct{}{}
			}
			return
		}
		runtime.GC()

		dir.UpdateStats(make(fs.HardLinkedItems, 10))

		if ui.ShowProgress {
			doneChan <- struct{}{}
		}
	}()

	wait.Wait()

	if err != nil {
		return err
	}

	if ui.summarize {
		ui.printTotalItem(dir)
	} else {
		ui.showDir(dir)
	}

	return nil
}

func (ui *UI) showReadingProgress(doneChan chan struct{}) {
	emptyRow := "\r"
	for j := 0; j < 40; j++ {
		emptyRow += " "
	}

	i := 0
	for {
		fmt.Fprint(ui.output, emptyRow)

		select {
		case <-doneChan:
			fmt.Fprint(ui.output, "\r")
			return
		default:
		}

		fmt.Fprintf(ui.output, "\r %s ", string(progressRunes[i]))
		fmt.Fprint(ui.output, "Reading analysis from file...")

		time.Sleep(100 * time.Millisecond)
		i++
		i %= progressRunesCount
	}
}

func (ui *UI) updateProgress(updateStatsDone <-chan struct{}) {
	emptyRow := "\r"
	for j := 0; j < 100; j++ {
		emptyRow += " "
	}

	progressChan := ui.Analyzer.GetProgressChan()
	analysisDoneChan := ui.Analyzer.GetDone()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var progress common.CurrentProgress

	i := 0
	for {
		select {
		case <-ticker.C:
			select {
			case progress = <-progressChan:
				fmt.Fprint(ui.output, emptyRow)
				fmt.Fprintf(ui.output, "\r %s ", string(progressRunes[i]))
				fmt.Fprint(ui.output, "Scanning... Total items: "+
					ui.red.Sprint(common.FormatNumber(int64(progress.ItemCount)))+
					" size: "+
					ui.formatSize(progress.TotalSize))
			default:
				// Update only the spinner without clearing the line
				fmt.Fprintf(ui.output, "\r %s ", string(progressRunes[i]))
			}
			i++
			i %= progressRunesCount
		case <-analysisDoneChan:
			ticker.Stop()
			fmt.Fprint(ui.output, emptyRow)
			for {
				fmt.Fprint(ui.output, emptyRow)
				fmt.Fprintf(ui.output, "\r %s ", string(progressRunes[i]))
				fmt.Fprint(ui.output, "Calculating disk usage...")
				time.Sleep(100 * time.Millisecond)
				i++
				i %= progressRunesCount

				select {
				case <-updateStatsDone:
					fmt.Fprint(ui.output, emptyRow)
					fmt.Fprint(ui.output, "\r")
					return
				default:
				}
			}
		}
	}
}

func (ui *UI) formatSize(size int64) string {
	if ui.noPrefix {
		return ui.orange.Sprintf("%d", size)
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
