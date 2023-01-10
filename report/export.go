package report

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
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
	output       io.Writer
	exportOutput io.Writer
	red          *color.Color
	orange       *color.Color
	writtenChan  chan struct{}
}

// CreateExportUI creates UI for stdout
func CreateExportUI(
	output io.Writer,
	exportOutput io.Writer,
	useColors bool,
	showProgress bool,
	constGC bool,
	useSIPrefix bool,
) *UI {
	ui := &UI{
		UI: &common.UI{
			ShowProgress: showProgress,
			Analyzer:     analyze.CreateAnalyzer(),
			ConstGC:      constGC,
			UseSIPrefix:  useSIPrefix,
		},
		output:       output,
		exportOutput: exportOutput,
		writtenChan:  make(chan struct{}),
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

// ListDevices lists mounted devices and shows their disk usage
func (ui *UI) ListDevices(getter device.DevicesInfoGetter) error {
	return errors.New("Exporting devices list is not supported")
}

// ReadAnalysis reads analysis report from JSON file
func (ui *UI) ReadAnalysis(input io.Reader) error {
	return errors.New("Reading analysis is not possible while exporting")
}

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, _ fs.Item) error {
	var (
		dir         fs.Item
		wait        sync.WaitGroup
		waitWritten sync.WaitGroup
		err         error
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
		dir = ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.ConstGC)
		dir.UpdateStats(make(fs.HardLinkedItems, 10))
	}()

	wait.Wait()

	sort.Sort(sort.Reverse(dir.GetFiles()))

	var buff bytes.Buffer

	buff.Write([]byte(`[1,2,{"progname":"gdu","progver":"`))
	buff.Write([]byte(build.Version))
	buff.Write([]byte(`","timestamp":`))
	buff.Write([]byte(strconv.FormatInt(time.Now().Unix(), 10)))
	buff.Write([]byte("},\n"))

	if err = dir.EncodeJSON(&buff, true); err != nil {
		return err
	}
	if _, err = buff.Write([]byte("]\n")); err != nil {
		return err
	}
	if _, err = buff.WriteTo(ui.exportOutput); err != nil {
		return err
	}

	switch f := ui.exportOutput.(type) {
	case *os.File:
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
				ui.formatSize(progress.TotalSize))
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
