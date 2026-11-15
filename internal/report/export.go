package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dundee/gdu/v6/build"
	"github.com/dundee/gdu/v6/internal/common"
	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/session"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/fatih/color"
)

const clearTerminalLine = "\r\x1b[2K"

// UI struct
type UI struct {
	*common.UI
	output           io.Writer
	exportOutput     io.Writer
	red              *color.Color
	orange           *color.Color
	progressRunes    []rune
	session          *session.Session
	outputAttributes fs.JSONAttributes
	top              int
	depth            int
	summarize        bool
}

// CreateExportUI returns the UI exporting the analysis to exportOutput as
// JSON, telling the progress on output.
func CreateExportUI(output, exportOutput io.Writer, analyzer analyze.Analyzer, s *config.Settings) *UI {
	ui := &UI{
		UI:               common.NewUI(analyzer, s),
		output:           output,
		exportOutput:     exportOutput,
		outputAttributes: s.Export.Attributes,
		top:              s.Display.Top,
		depth:            s.Display.Depth,
		summarize:        s.Display.Summarize,
	}
	if !s.Display.SIPrefix {
		ui.SetBlockSizeFromEnvironment()
	}
	ui.progressRunes = common.SpinnerRunes(s.Display.NoUnicode)
	ui.red = common.BoldColor(color.FgRed, s.Display.Colors)
	ui.orange = common.BoldColor(color.FgYellow, s.Display.Colors)
	ui.session = session.New(ui.UI)

	return ui
}

// StartUILoop stub
func (ui *UI) StartUILoop() error {
	return nil
}

// ListDevices lists mounted devices and shows their disk usage
func (ui *UI) ListDevices(getter device.DevicesInfoGetter) error {
	return errors.New("exporting devices list is not supported")
}

// ReadAnalysis reads analysis report from JSON file
func (ui *UI) ReadAnalysis(input io.Reader) error {
	return errors.New("reading analysis is not possible while exporting")
}

// ReadFromStorage exports the analysis of path stored by an earlier scan with
// --db.
func (ui *UI) ReadFromStorage(path string) error {
	dir, err := ui.session.Open(path)
	if err != nil {
		return err
	}
	return ui.exportDir(dir)
}

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, _ fs.Item) error {
	dir := ui.session.Scan(context.Background(), []string{path}, ui.scanObserver())
	return ui.exportDir(dir)
}

// AnalyzePaths is not supported by the export UI: the ncdu-compatible export
// format holds exactly one root directory, so there is nowhere to record a
// virtual dir grouping several of them.
func (ui *UI) AnalyzePaths(paths []string) error {
	if len(paths) == 1 {
		return ui.AnalyzePath(paths[0], nil)
	}
	return errors.New("exporting more than one directory is not supported")
}

// exportedDir copies the attributes of dir that the export can emit. The
// export filters can receive any fs.Item implementation (a plain scan,
// analyses stored in SQLite/BadgerDB, entries inside browsed archives, ...),
// so the copy reads everything through the interface instead of type-asserting
// *analyze.Dir. BasePath is not part of the interface and is rebuilt from
// GetPath, so the top-level entry keeps its full path in the export.
func exportedDir(dir fs.Item) *analyze.Dir {
	return &analyze.Dir{
		File: &analyze.File{
			Name:  dir.GetName(),
			Flag:  dir.GetFlag(),
			Mtime: dir.GetMtime(),
			Size:  dir.GetSize(),
			Usage: dir.GetUsage(),
		},
		BasePath:  filepath.Dir(dir.GetPath()),
		ItemCount: dir.GetItemCount(),
		Dev:       analyze.Device(dir),
	}
}

// exportedFile copies the attributes of a file the export can emit, reading
// them through the fs.Item interface for the same reason as exportedDir.
func exportedFile(file, parent fs.Item) *analyze.File {
	copied := &analyze.File{
		Name:   file.GetName(),
		Flag:   file.GetFlag(),
		Size:   file.GetSize(),
		Usage:  file.GetUsage(),
		Mtime:  file.GetMtime(),
		Mli:    file.GetMultiLinkedInode(),
		Parent: parent,
	}
	if symlink, ok := file.(fs.SymlinkItem); ok {
		copied.Symlink = symlink.GetSymlinkTarget()
	}
	return copied
}

func (ui *UI) topDir(dir fs.Item) *analyze.Dir {
	// the export UI is never told about --show-apparent-size, so its top-N
	// ranking stays on apparent size
	files := analyze.CollectTopFiles(dir, ui.top, fs.SortByApparentSize)

	topDir := exportedDir(dir)
	for _, f := range files {
		topDir.AddFile(exportedFile(f, topDir))
	}
	topDir.UpdateStats(false)
	return topDir
}

// limitDirByDepth returns a copy of dir, a directory, holding only the
// entries depth levels below the top of the export at most.
func (ui *UI) limitDirByDepth(dir fs.Item, currentDepth int) *analyze.Dir {
	limited := exportedDir(dir)
	if currentDepth == ui.depth {
		return limited
	}
	for f := range dir.GetFiles(fs.SortBySize, fs.SortDesc) {
		if f.IsDir() {
			child := ui.limitDirByDepth(f, currentDepth+1)
			child.SetParent(limited)
			limited.AddFile(child)
		} else if currentDepth+1 <= ui.depth {
			limited.AddFile(exportedFile(f, limited))
		}
	}
	return limited
}

func (ui *UI) summarizeDir(dir fs.Item) *analyze.Dir {
	return exportedDir(dir)
}

func (ui *UI) exportDir(dir analyze.Node) error {
	stop := ui.showWritingProgress()
	defer stop()

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
	case ui.depth > 0 && dir.IsDir():
		dir = ui.limitDirByDepth(dir, 0)
	}

	if err := dir.EncodeJSON(&buff, true, ui.outputAttributes); err != nil {
		return err
	}
	if _, err = buff.Write([]byte("]\n")); err != nil {
		return err
	}
	_, err = buff.WriteTo(ui.exportOutput)
	// closing finishes the export also after a failed write, which it then
	// discards
	if closer, ok := ui.exportOutput.(io.Closer); ok {
		err = errors.Join(err, closer.Close())
	}
	return err
}

// scanObserver returns the observer showing the progress of a scan, or nil
// when no progress is shown.
func (ui *UI) scanObserver() session.Observer {
	if !ui.ShowProgress {
		return nil
	}
	i := 0
	return func(p session.Progress) {
		fmt.Fprint(ui.output, clearTerminalLine)
		fmt.Fprintf(ui.output, "\r %s ", string(ui.progressRunes[i]))
		if p.Phase == session.Finalizing {
			fmt.Fprint(ui.output, "Calculating disk usage...")
		} else {
			fmt.Fprint(ui.output, "Scanning... Total items: "+
				ui.red.Sprint(common.FormatNumber(p.ItemCount))+
				" size: "+
				ui.formatSize(p.TotalUsage))
		}
		i = (i + 1) % len(ui.progressRunes)
	}
}

// showWritingProgress spins until the returned function is called, which
// clears the line again.
func (ui *UI) showWritingProgress() (stop func()) {
	if !ui.ShowProgress {
		return func() {}
	}
	quit := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; ; i = (i + 1) % len(ui.progressRunes) {
			fmt.Fprint(ui.output, clearTerminalLine)
			fmt.Fprintf(ui.output, "\r %s Writing output file...", string(ui.progressRunes[i]))
			select {
			case <-quit:
				fmt.Fprint(ui.output, clearTerminalLine)
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		close(quit)
		<-stopped
	}
}

func (ui *UI) formatSize(size int64) string {
	if formatted, ok := ui.FormatBlockSize(size); ok {
		return ui.orange.Sprint(formatted)
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
