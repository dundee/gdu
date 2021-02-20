package stdout

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dundee/gdu/analyze"
	"github.com/dundee/gdu/device"
	"github.com/fatih/color"
)

// UI struct
type UI struct {
	analyzer         analyze.Analyzer
	output           io.Writer
	ignoreDirPaths   map[string]bool
	useColors        bool
	showProgress     bool
	showApparentSize bool
	red              *color.Color
	orange           *color.Color
	blue             *color.Color
}

// CreateStdoutUI creates UI for stdout
func CreateStdoutUI(output io.Writer, useColors bool, showProgress bool, showApparentSize bool) *UI {
	ui := &UI{
		output:           output,
		useColors:        useColors,
		showProgress:     showProgress,
		showApparentSize: showApparentSize,
		analyzer:         analyze.CreateAnalyzer(),
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
	if ui.useColors {
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
func (ui *UI) AnalyzePath(path string, _ *analyze.File) {
	abspath, _ := filepath.Abs(path)
	var dir *analyze.File

	var wait sync.WaitGroup

	if ui.showProgress {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ui.updateProgress()
		}()
	}

	wait.Add(1)
	go func() {
		defer wait.Done()
		dir = ui.analyzer.AnalyzeDir(abspath, ui.ShouldDirBeIgnored)
	}()

	wait.Wait()

	sort.Sort(dir.Files)

	var lineFormat string
	if ui.useColors {
		lineFormat = "%s %20s %s\n"
	} else {
		lineFormat = "%s %9s %s\n"
	}

	var size int64

	for _, file := range dir.Files {
		if ui.showApparentSize {
			size = file.Size
		} else {
			size = file.Usage
		}

		if file.IsDir {
			fmt.Fprintf(ui.output,
				lineFormat,
				string(file.Flag),
				ui.formatSize(size),
				ui.blue.Sprintf("/"+file.Name))
		} else {
			fmt.Fprintf(ui.output,
				lineFormat,
				string(file.Flag),
				ui.formatSize(size),
				file.Name)
		}
	}
}

// SetIgnoreDirPaths sets paths to ignore
func (ui *UI) SetIgnoreDirPaths(paths []string) {
	ui.ignoreDirPaths = make(map[string]bool, len(paths))
	for _, path := range paths {
		ui.ignoreDirPaths[path] = true
	}
}

// ShouldDirBeIgnored returns true if given path should be ignored
func (ui *UI) ShouldDirBeIgnored(path string) bool {
	return ui.ignoreDirPaths[path]
}

func (ui *UI) updateProgress() {
	emptyRow := "\r"
	for j := 0; j < 100; j++ {
		emptyRow += " "
	}

	progressRunes := []rune(`⠇⠏⠋⠙⠹⠸⠼⠴⠦⠧`)

	progress := ui.analyzer.GetProgress()

	i := 0
	for {
		progress.Mutex.Lock()

		fmt.Fprint(ui.output, emptyRow)

		if progress.Done {
			fmt.Fprint(ui.output, "\r")
			return
		}

		fmt.Fprintf(ui.output, "\r %s ", string(progressRunes[i]))

		fmt.Fprint(ui.output, "Scanning... Total items: "+
			ui.red.Sprint(progress.ItemCount)+
			" size: "+
			ui.formatSize(progress.TotalSize))
		progress.Mutex.Unlock()

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
