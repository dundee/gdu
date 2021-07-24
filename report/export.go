package report

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
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
func CreateExportUI(output io.Writer, exportOutput io.Writer, useColors bool, showProgress bool) *UI {
	ui := &UI{
		UI: &common.UI{
			ShowProgress: showProgress,
			Analyzer:     analyze.CreateAnalyzer(),
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
func (ui *UI) AnalyzePath(path string, _ *analyze.Dir) error {
	var (
		dir         *analyze.Dir
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
		dir = ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc())
	}()

	wait.Wait()

	sort.Sort(dir.Files)

	var buff bytes.Buffer

	buff.Write([]byte(`[1,2,{"progname":"gdu","progver":"`))
	buff.Write([]byte(build.Version))
	buff.Write([]byte(`","timestamp":`))
	buff.Write([]byte(fmt.Sprint(time.Now().Unix())))
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
		f.Close()
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
	doneChan := ui.Analyzer.GetDoneChan()

	var progress analyze.CurrentProgress

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
