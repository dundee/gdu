package stdout

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dundee/gdu/v4/analyze"
	"github.com/dundee/gdu/v4/common"
	"github.com/dundee/gdu/v4/device"
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

// CreateStdoutUI creates UI for stdout
func CreateStdoutUI(output io.Writer, useColors bool, showProgress bool, showApparentSize bool) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowProgress:     showProgress,
			ShowApparentSize: showApparentSize,
			Analyzer:         analyze.CreateAnalyzer(),
			PathChecker:      os.Stat,
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
	devices := check getter.GetDevicesInfo()

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
	abspath, _ := filepath.Abs(path)

	check ui.PathChecker(abspath)

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
		dir = ui.Analyzer.AnalyzeDir(abspath, ui.CreateIgnoreFunc())
	}()

	wait.Wait()

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

	return nil
}

func (ui *UI) updateProgress() {
	emptyRow := "\r"
	for j := 0; j < 100; j++ {
		emptyRow += " "
	}

	progressRunes := []rune(`⠇⠏⠋⠙⠹⠸⠼⠴⠦⠧`)

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
			ui.red.Sprint(progress.ItemCount)+
			" size: "+
			ui.formatSize(progress.TotalSize))

		time.Sleep(100 * time.Millisecond)
		i++
		i %= 10
	}
}

func (ui *UI) formatSize(size int64) string {
	switch {
	case size > 1e12:
		return ui.orange.Sprintf("%.1f", float64(size)/math.Pow(2, 40)) + " TiB"
	case size > 1e9:
		return ui.orange.Sprintf("%.1f", float64(size)/math.Pow(2, 30)) + " GiB"
	case size > 1e6:
		return ui.orange.Sprintf("%.1f", float64(size)/math.Pow(2, 20)) + " MiB"
	case size > 1e3:
		return ui.orange.Sprintf("%.1f", float64(size)/math.Pow(2, 10)) + " KiB"
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
