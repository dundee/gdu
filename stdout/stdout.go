package stdout

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dundee/gdu/analyze"
)

// UI struct
type UI struct {
	ignoreDirPaths map[string]bool
	useColors      bool
	showProgress   bool
}

// CreateStdoutUI creates UI for stdout
func CreateStdoutUI(useColors bool, showProgress bool) *UI {
	ui := &UI{
		useColors:    useColors,
		showProgress: showProgress,
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

	print("\r")
	prefix := ""
	for _, file := range dir.Files {
		if file.IsDir {
			prefix = "/"
		}
		fmt.Printf("%10s %s%s\n",
			formatSize(file.Size),
			prefix,
			file.Name)
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

	i := 0
	for {
		progress.Mutex.Lock()

		print(emptyRow)

		if progress.Done {
			return
		}

		switch i {
		case 0:
			print("\r | ")
			break
		case 1:
			print("\r / ")
			break
		case 2:
			print("\r - ")
			break
		case 3:
			print("\r \\ ")
			break
		}

		print("Scanning... Total items: " +
			fmt.Sprint(progress.ItemCount) +
			" size: " +
			formatSize(progress.TotalSize))
		progress.Mutex.Unlock()

		time.Sleep(100 * time.Millisecond)
		i++
		i %= 4
	}
}

func formatSize(size int64) string {
	if size > 1e12 {
		return fmt.Sprintf("%.1f TiB", float64(size)/math.Pow(2, 40))
	} else if size > 1e9 {
		return fmt.Sprintf("%.1f GiB", float64(size)/math.Pow(2, 30))
	} else if size > 1e6 {
		return fmt.Sprintf("%.1f MiB", float64(size)/math.Pow(2, 20))
	} else if size > 1e3 {
		return fmt.Sprintf("%.1f KiB", float64(size)/math.Pow(2, 10))
	}
	return fmt.Sprintf("%d B", size)
}
