package stdout

import (
	"fmt"
	"io"
	"math"
	"runtime"
	"sort"
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
	*common.UI
	output    io.Writer
	red       *color.Color
	orange    *color.Color
	blue      *color.Color
	summarize bool
	noPrefix  bool
}

var progressRunes = []rune(`⠇⠏⠋⠙⠹⠸⠼⠴⠦⠧`)

// CreateStdoutUI creates UI for stdout
func CreateStdoutUI(
	output io.Writer,
	useColors bool,
	showProgress bool,
	showApparentSize bool,
	showRelativeSize bool,
	summarize bool,
	constGC bool,
	useSIPrefix bool,
	noPrefix bool,
) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowProgress:     showProgress,
			ShowApparentSize: showApparentSize,
			ShowRelativeSize: showRelativeSize,
			Analyzer:         analyze.CreateAnalyzer(),
			ConstGC:          constGC,
			UseSIPrefix:      useSIPrefix,
		},
		output:    output,
		summarize: summarize,
		noPrefix:  noPrefix,
	}

	ui.red = color.New(color.FgRed).Add(color.Bold)
	ui.orange = color.New(color.FgYellow).Add(color.Bold)
	ui.blue = color.New(color.FgBlue).Add(color.Bold)

	if !useColors {
		color.NoColor = true
	}

	return ui
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

	maxDeviceNameLenght := maxInt(maxLength(
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
		maxDeviceNameLenght,
		sizeLength,
		sizeLength,
		sizeLength,
		percentLength,
	)

	fmt.Fprintf(
		ui.output,
		fmt.Sprintf("%%%ds %%9s %%9s %%9s %%5s %%s\n", maxDeviceNameLenght),
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
		dir  fs.Item
		wait sync.WaitGroup
	)

	if ui.ShowProgress {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ui.updateProgress()
		}()
	}

	wait.Add(1)
	go func() {
		defer wait.Done()
		dir = ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.ConstGC)
		dir.UpdateStats(make(fs.HardLinkedItems, 10))
	}()

	wait.Wait()

	if ui.summarize {
		ui.printTotalItem(dir)
	} else {
		ui.showDir(dir)
	}

	return nil
}

func (ui *UI) showDir(dir fs.Item) {
	sort.Sort(sort.Reverse(dir.GetFiles()))

	for _, file := range dir.GetFiles() {
		ui.printItem(file)
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
	if ui.UseColors {
		lineFormat = "%s %20s %s\n"
	} else {
		lineFormat = "%s %9s %s\n"
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
			string(file.GetFlag()),
			ui.formatSize(size),
			ui.blue.Sprintf("/"+file.GetName()))
	} else {
		fmt.Fprintf(ui.output,
			lineFormat,
			string(file.GetFlag()),
			ui.formatSize(size),
			file.GetName())
	}
}

// ReadAnalysis reads analysis report from JSON file
func (ui *UI) ReadAnalysis(input io.Reader) error {
	var (
		dir      *analyze.Dir
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
		i %= 10
	}
}

func (ui *UI) updateProgress() {
	emptyRow := "\r"
	for j := 0; j < 100; j++ {
		emptyRow += " "
	}

	progressChan := ui.Analyzer.GetProgressChan()
	doneChan := ui.Analyzer.GetDone()

	var progress common.CurrentProgress

	i := 0
	for {
		fmt.Fprint(ui.output, emptyRow)

		select {
		case progress = <-progressChan:
		case <-doneChan:
			fmt.Fprint(ui.output, "\r")
			return
		}

		fmt.Fprintf(ui.output, "\r %s ", string(progressRunes[i]))

		fmt.Fprint(ui.output, "Scanning... Total items: "+
			ui.red.Sprint(common.FormatNumber(int64(progress.ItemCount)))+
			" size: "+
			ui.formatSize(progress.TotalSize))

		time.Sleep(100 * time.Millisecond)
		i++
		i %= 10
	}
}

func (ui *UI) formatSize(size int64) string {
	if ui.noPrefix {
		return ui.orange.Sprintf("%d", size)
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

func maxInt(x int, y int) int {
	if x > y {
		return x
	}
	return y
}
