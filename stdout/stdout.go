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
	"github.com/gookit/color"
)

// UI struct
type UI struct {
	output         io.Writer
	ignoreDirPaths map[string]bool
	useColors      bool
	showProgress   bool
	red            color.Style
	orange         color.Style
	blue           color.Style
}

// CreateStdoutUI creates UI for stdout
func CreateStdoutUI(output io.Writer, useColors bool, showProgress bool) *UI {
	ui := &UI{
		output:       output,
		useColors:    useColors,
		showProgress: showProgress,
	}

	ui.red = color.Style{color.FgRed, color.OpBold}
	ui.orange = color.Style{color.FgYellow, color.OpBold}
	ui.blue = color.Style{color.FgBlue, color.OpBold}

	if !useColors {
		color.Disable()
	}

	return ui
}

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, analyzer analyze.Analyzer) {
	abspath, _ := filepath.Abs(path)
	var dir *analyze.File

	progress := &analyze.CurrentProgress{
		Mutex:     &sync.Mutex{},
		Done:      false,
		ItemCount: 0,
		TotalSize: int64(0),
	}
	var wait sync.WaitGroup

	if ui.showProgress {
		wait.Add(1)
		go func() {
			ui.updateProgress(progress)
			wait.Done()
		}()
	}

	wait.Add(1)
	go func() {
		dir = analyzer(abspath, progress, ui.ShouldDirBeIgnored)
		wait.Done()
	}()

	wait.Wait()

	sort.Sort(dir.Files)

	var lineFormat string
	if ui.useColors {
		lineFormat = "%20s %s\n"
	} else {
		lineFormat = "%9s %s\n"
	}

	for _, file := range dir.Files {
		if file.IsDir {
			fmt.Fprintf(ui.output,
				lineFormat,
				ui.formatSize(file.Size),
				ui.blue.Sprintf("/"+file.Name))
		} else {
			fmt.Fprintf(ui.output,
				lineFormat,
				ui.formatSize(file.Size),
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

func (ui *UI) updateProgress(progress *analyze.CurrentProgress) {
	emptyRow := "\r"
	for j := 0; j < 100; j++ {
		emptyRow += " "
	}

	progressRunes := []rune(`⠇⠏⠋⠙⠹⠸⠼⠴⠦⠧`)

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
	if size > 1e12 {
		return ui.orange.Sprintf("%.1f", float64(size)/math.Pow(2, 40)) + " TiB"
	} else if size > 1e9 {
		return ui.orange.Sprintf("%.1f", float64(size)/math.Pow(2, 30)) + " GiB"
	} else if size > 1e6 {
		return ui.orange.Sprintf("%.1f", float64(size)/math.Pow(2, 20)) + " MiB"
	} else if size > 1e3 {
		return ui.orange.Sprintf("%.1f", float64(size)/math.Pow(2, 10)) + " KiB"
	}
	return ui.orange.Sprintf("%d", size) + " B"
}
