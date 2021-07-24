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
	"github.com/dundee/gdu/v5/report"
	"github.com/fatih/color"
)

// UI struct
type UI struct {
	*common.UI
	output io.Writer
	red    *color.Color
	orange *color.Color
	blue   *color.Color
}

var progressRunes = []rune(`⠇⠏⠋⠙⠹⠸⠼⠴⠦⠧`)

// CreateStdoutUI creates UI for stdout
func CreateStdoutUI(output io.Writer, useColors bool, showProgress bool, showApparentSize bool) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowProgress:     showProgress,
			ShowApparentSize: showApparentSize,
			Analyzer:         analyze.CreateAnalyzer(),
		},
		output: output,
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
func (ui *UI) AnalyzePath(path string, _ *analyze.Dir) error {
	var (
		dir  *analyze.Dir
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
		dir = ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc())
	}()

	wait.Wait()

	ui.showDir(dir)

	return nil
}

func (ui *UI) showDir(dir *analyze.Dir) {
	sort.Sort(dir.Files)

	var lineFormat string
	if ui.UseColors {
		lineFormat = "%s %20s %s\n"
	} else {
		lineFormat = "%s %9s %s\n"
	}

	var size int64

	for _, file := range dir.Files {
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

		links := make(analyze.AlreadyCountedHardlinks, 10)
		dir.UpdateStats(links)

		if ui.ShowProgress {
			doneChan <- struct{}{}
		}
	}()

	wait.Wait()

	if err != nil {
		return err
	}

	ui.showDir(dir)

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
	doneChan := ui.Analyzer.GetDoneChan()

	var progress analyze.CurrentProgress

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
	fsize := float64(size)

	switch {
	case fsize >= common.EB:
		return ui.orange.Sprintf("%.1f", fsize/common.EB) + " EiB"
	case fsize >= common.PB:
		return ui.orange.Sprintf("%.1f", fsize/common.PB) + " PiB"
	case fsize >= common.TB:
		return ui.orange.Sprintf("%.1f", fsize/common.TB) + " TiB"
	case fsize >= common.GB:
		return ui.orange.Sprintf("%.1f", fsize/common.GB) + " GiB"
	case fsize >= common.MB:
		return ui.orange.Sprintf("%.1f", fsize/common.MB) + " MiB"
	case fsize >= common.KB:
		return ui.orange.Sprintf("%.1f", fsize/common.KB) + " KiB"
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
