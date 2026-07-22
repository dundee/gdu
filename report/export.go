package report

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/fatih/color"
)

// UI struct
type UI struct {
	*common.UI
	output           io.Writer
	exportOutput     io.Writer
	red              *color.Color
	orange           *color.Color
	writtenChan      chan struct{}
	outputAttributes fs.JSONAttributes
	top              int
	depth            int
	summarize        bool
}

// CreateExportUI creates UI for stdout
func CreateExportUI(
	output io.Writer,
	exportOutput io.Writer,
	useColors bool,
	showProgress bool,
	useSIPrefix bool,
	top int,
	depth int,
	summarize bool,
	outputAttributes fs.JSONAttributes,
) *UI {
	ui := &UI{
		UI: &common.UI{
			ShowProgress: showProgress,
			Analyzer:     analyze.CreateAnalyzer(),
			UseSIPrefix:  useSIPrefix,
		},
		output:           output,
		exportOutput:     exportOutput,
		writtenChan:      make(chan struct{}),
		outputAttributes: outputAttributes,
		top:              top,
		depth:            depth,
		summarize:        summarize,
	}
	ui.red = color.New(color.FgRed).Add(color.Bold)
	ui.orange = color.New(color.FgYellow).Add(color.Bold)

	if !useColors {
		color.NoColor = true
	}

	return ui
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
	return errors.New("exporting devices list is not supported")
}

// ReadAnalysis reads analysis report from JSON file
func (ui *UI) ReadAnalysis(input io.Reader) error {
	return errors.New("reading analysis is not possible while exporting")
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

	var waitWritten sync.WaitGroup
	if ui.ShowProgress {
		waitWritten.Add(1)
		go func() {
			defer waitWritten.Done()
			ui.updateProgress()
		}()
	}

	return ui.exportDir(dir, &waitWritten)
}

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, _ fs.Item) error {
	var (
		dir         fs.Item
		wait        sync.WaitGroup
		waitWritten sync.WaitGroup
	)

	if ui.ShowProgress {
		waitWritten.Add(1)
		go func() {
			defer waitWritten.Done()
			ui.updateProgress()
		}()
	}

	wait.Add(1)
	go func() {
		defer wait.Done()
		dir = ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter())
		if ui.IsFilteringFiles() {
			dir.UpdateStatsWithFileFiltering(make(fs.HardLinkedItems, 10))
		} else {
			dir.UpdateStats(make(fs.HardLinkedItems, 10))
		}
	}()

	wait.Wait()

	return ui.exportDir(dir, &waitWritten)
}

func (ui *UI) topDir(dir fs.Item) fs.Item {
	files := analyze.CollectTopFiles(dir, ui.top)

	topDir := &analyze.Dir{
		File: &analyze.File{
			Name:  dir.GetName(),
			Mtime: dir.GetMtime(),
		},
	}
	if d, ok := dir.(*analyze.Dir); ok {
		topDir.BasePath = d.BasePath
	}
	for _, f := range files {
		file := *f.(*analyze.File)
		file.Parent = topDir
		topDir.AddFile(&file)
	}
	topDir.UpdateStats(make(fs.HardLinkedItems, 10))
	return topDir
}

func (ui *UI) limitDirByDepth(dir fs.Item, currentDepth int) fs.Item {
	if d, ok := dir.(*analyze.Dir); ok {
		limited := &analyze.Dir{
			File: &analyze.File{
				Name:   d.GetName(),
				Mtime:  d.GetMtime(),
				Parent: d.GetParent(),
				Size:   d.GetSize(),
				Usage:  d.GetUsage(),
			},
			BasePath:  d.BasePath,
			ItemCount: d.ItemCount,
		}
		if currentDepth == ui.depth {
			return limited
		}
		for f := range d.GetFiles(fs.SortBySize, fs.SortDesc) {
			if f.IsDir() {
				child := ui.limitDirByDepth(f, currentDepth+1)
				if child != nil {
					child.SetParent(limited)
					limited.AddFile(child)
				}
			} else if currentDepth+1 <= ui.depth {
				file := *f.(*analyze.File)
				file.Parent = limited
				limited.AddFile(&file)
			}
		}
		return limited
	}

	return dir
}

func (ui *UI) summarizeDir(dir fs.Item) fs.Item {
	summary := &analyze.Dir{
		File: &analyze.File{
			Name:  dir.GetName(),
			Mtime: dir.GetMtime(),
		},
	}
	if d, ok := dir.(*analyze.Dir); ok {
		summary.BasePath = d.BasePath
		summary.ItemCount = d.ItemCount
		summary.Size = d.GetSize()
		summary.Usage = d.GetUsage()
	}
	return summary
}

func (ui *UI) exportDir(dir fs.Item, waitWritten *sync.WaitGroup) error {
	// Sorting is now handled by GetFiles with sort parameters

	var (
		buff bytes.Buffer
		err  error
	)

	buff.Write([]byte(`[1,2,{"progname":"gdu","progver":"`))
	buff.Write([]byte(build.Version))
	buff.Write([]byte(`","timestamp":`))
	buff.Write([]byte(strconv.FormatInt(time.Now().Unix(), 10)))
	buff.Write([]byte("},\n"))

	switch {
	case ui.summarize:
		dir = ui.summarizeDir(dir)
	case ui.top > 0:
		dir = ui.topDir(dir)
	case ui.depth > 0:
		dir = ui.limitDirByDepth(dir, 0)
	}

	if err := dir.EncodeJSON(&buff, true, ui.outputAttributes); err != nil {
		return err
	}
	if _, err = buff.Write([]byte("]\n")); err != nil {
		return err
	}
	if _, err = buff.WriteTo(ui.exportOutput); err != nil {
		return err
	}

	if f, ok := ui.exportOutput.(*os.File); ok {
		err = f.Close()
		if err != nil {
			return err
		}
	}

	if ui.ShowProgress {
		ui.writtenChan <- struct{}{}
		waitWritten.Wait()
	}

	return nil
}

func (ui *UI) updateProgress() {
	waitingForWrite := false

	emptyRow := "\r"
	for j := 0; j < 100; j++ {
		emptyRow += " "
	}

	progressRunes := []rune(`⠇⠏⠋⠙⠹⠸⠼⠴⠦⠧`)

	doneChan := ui.Analyzer.GetDone()

	i := 0
	for {
		fmt.Fprint(ui.output, emptyRow)

		progress := ui.Analyzer.GetProgress()

		select {
		case <-doneChan:
			fmt.Fprint(ui.output, "\r")
			waitingForWrite = true
		case <-ui.writtenChan:
			fmt.Fprint(ui.output, "\r")
			return
		default:
		}

		fmt.Fprintf(ui.output, "\r %s ", string(progressRunes[i]))

		if waitingForWrite {
			fmt.Fprint(ui.output, "Writing output file...")
		} else {
			fmt.Fprint(ui.output, "Scanning... Total items: "+
				ui.red.Sprint(common.FormatNumber(int64(progress.ItemCount)))+
				" size: "+
				ui.formatSize(progress.TotalUsage))
		}

		time.Sleep(100 * time.Millisecond)
		i++
		i %= 10
	}
}

func (ui *UI) formatSize(size int64) string {
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
