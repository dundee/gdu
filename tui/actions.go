package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/internal/session"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
)

const (
	defaultLinesCount = 500
	linesThreshold    = 20
)

// ListDevices lists mounted devices and shows their disk usage
func (ui *UI) ListDevices(getter device.DevicesInfoGetter) error {
	var err error
	ui.getter = getter
	ui.devices, err = getter.GetDevicesInfo()
	if err != nil {
		return err
	}

	ui.showDevices()

	return nil
}

// showScanProgressModal builds and shows the modal that reports scan progress.
func (ui *UI) showScanProgressModal(title string) {
	ui.progress = tview.NewTextView().SetText("Scanning...")
	ui.progress.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	ui.progress.SetTitle(title)
	ui.progress.SetDynamicColors(true)

	innerFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(ui.progress, 8, 1, false)

	if ui.currentDeviceSize > 0 && ui.showDiskProgressBar {
		ui.progressBar = NewProgressBar()
		ui.progressBar.SetBorder(true)
		ui.progressBar.SetTitle(" Progress ")
		ui.progressBar.SetUseColor(ui.UseColors)
		innerFlex.AddItem(ui.progressBar, 3, 1, false)
	}

	innerFlex.AddItem(nil, 0, 1, false)

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(innerFlex, 0, 50, false).
		AddItem(nil, 0, 1, false)

	ui.pages.AddPage("progress", flex, true, true)
	ui.progressFlex = flex
}

// AnalyzePath analyzes recursively disk usage for given path
func (ui *UI) AnalyzePath(path string, _ fs.Item) error {
	ui.startScan(" Scanning... ", func(ctx context.Context, observe session.Observer) fs.Item {
		return ui.session.Scan(ctx, []string{path}, observe)
	})
	return nil
}

// AnalyzePaths scans several roots one after another and presents them under a
// virtual top level dir, so that the rest of the UI keeps working on the single
// rooted tree it expects.
func (ui *UI) AnalyzePaths(paths []string) error {
	if len(paths) == 1 {
		return ui.AnalyzePath(paths[0], nil)
	}
	ui.startScan(scanProgressTitle(1, len(paths)), func(ctx context.Context, observe session.Observer) fs.Item {
		return ui.session.Scan(ctx, paths, observe)
	})
	return nil
}

// rescan scans dir again and shows what the tree then holds in its place.
func (ui *UI) rescan(dir fs.Item) {
	title := " Scanning... "
	if roots := len(analyze.ScannedRootPaths(dir)); analyze.IsVirtualRootDir(dir) && roots > 1 {
		title = scanProgressTitle(1, roots)
	}
	ui.startScan(title, func(ctx context.Context, observe session.Observer) fs.Item {
		return ui.session.Rescan(ctx, dir, observe)
	})
}

// startScan shows the scan progress and runs scan in the background, then
// shows the directory scan returns. The scan can be stopped with cancelScan
// from the moment startScan returns.
func (ui *UI) startScan(title string, scan func(context.Context, session.Observer) fs.Item) {
	ui.showScanProgressModal(title)

	ctx, cancel := context.WithCancel(context.Background())
	ui.stopScan = cancel
	ui.scanStart = time.Now()
	ui.scanning = true
	ui.scanCancelled = false

	showDiskProgress := ui.currentDeviceSize > 0 && ui.showDiskProgressBar
	observe := ui.progressObserver(ui.currentDeviceSize, showDiskProgress)

	go func() {
		defer debug.FreeOSMemory()
		defer cancel()

		shown := scan(ctx, observe)
		topDirPath := ui.session.RootPath()
		if showDiskProgress {
			clearTerminalProgress()
		}

		ui.app.QueueUpdateDraw(func() {
			ui.topDirPath = topDirPath
			if showDiskProgress {
				ui.currentDeviceSize = 0
			}
			ui.scanning = false
			ui.scanCancelled = false
			// remember the longest scan so we can guard against accidental quits
			if d := time.Since(ui.scanStart); d > ui.scanDuration {
				ui.scanDuration = d
			}
			// the finished scan replaces any mid-scan preview
			ui.previewing = false
			ui.previewSavedDir = nil
			ui.currentDir = shown
			ui.showDir()
			ui.pages.RemovePage("progress")
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
}

func scanProgressTitle(current, total int) string {
	return fmt.Sprintf(" Scanning %d/%d... ", current, total)
}

// ReadAnalysis reads analysis report from JSON file
func (ui *UI) ReadAnalysis(input io.Reader) error {
	ui.progress = tview.NewTextView().SetText("Reading analysis from file...")
	ui.progress.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	ui.progress.SetTitle(" Reading... ")
	ui.progress.SetDynamicColors(true)

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 10, 1, false).
			AddItem(ui.progress, 8, 1, false).
			AddItem(nil, 10, 1, false), 0, 50, false).
		AddItem(nil, 0, 1, false)

	ui.pages.AddPage("progress", flex, true, true)

	go func() {
		dir, err := ui.session.Load(input)
		if err != nil {
			ui.app.QueueUpdateDraw(func() {
				ui.pages.RemovePage("progress")
				ui.showErr("Error reading file", err)
			})
			if ui.done != nil {
				ui.done <- struct{}{}
			}
			return
		}

		topDirPath := dir.GetPath()
		ui.app.QueueUpdateDraw(func() {
			ui.topDirPath = topDirPath
			ui.currentDir = dir
			ui.showDir()
			ui.pages.RemovePage("progress")
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()

	return nil
}

// ReadFromStorage shows the analysis of path stored by an earlier scan with
// --db.
func (ui *UI) ReadFromStorage(path string) error {
	dir, err := ui.session.Open(path)
	if err != nil {
		return err
	}

	ui.currentDir = dir
	ui.topDirPath = dir.GetPath()
	ui.showDir()
	return nil
}

func (ui *UI) showInfo() {
	if ui.currentDir == nil {
		return
	}

	var content, numberColor string
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(fs.Item)

	if ui.UseColors {
		numberColor = fmt.Sprintf(
			"[%s::b]",
			ui.resultRow.NumberColor,
		)
	} else {
		numberColor = defaultColorBold
	}

	linesCount := 12

	text := tview.NewTextView().SetDynamicColors(true)
	text.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	text.SetBorderColor(tcell.ColorDefault)
	text.SetTitle(" Item info ")

	content += "[::b]Name:[::-] "
	content += tview.Escape(selectedFile.GetName()) + "\n"
	content += "[::b]Path:[::-] "
	content += tview.Escape(
		strings.TrimPrefix(selectedFile.GetPath(), build.RootPathPrefix),
	) + "\n"
	content += "[::b]Type:[::-] " + selectedFile.GetType() + "\n"

	// Display symlink target
	if line := ui.symlinkTargetInfoLine(selectedFile); line != "" {
		content += line
		linesCount++
	}

	content += "\n"

	content += "   [::b]Disk usage:[::-] "
	content += numberColor + ui.formatSize(selectedFile.GetUsage(), false, true)
	content += fmt.Sprintf(" (%s%d[-::] B)", numberColor, selectedFile.GetUsage()) + "\n"
	content += "[::b]Apparent size:[::-] "
	content += numberColor + ui.formatSize(selectedFile.GetSize(), false, true)
	content += fmt.Sprintf(" (%s%d[-::] B)", numberColor, selectedFile.GetSize()) + "\n"

	if selectedFile.GetMultiLinkedInode() > 0 {
		linkedItems := ui.session.HardLinks(selectedFile.GetMultiLinkedInode())
		linesCount += 2 + len(linkedItems)
		content += "\nHard-linked files:\n"
		for _, linkedItem := range linkedItems {
			content += "\t" + linkedItem.GetPath() + "\n"
		}
	}

	text.SetText(content)

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, linesCount, 1, false).
			AddItem(nil, 0, 1, false), 80, 1, false).
		AddItem(nil, 0, 1, false)

	ui.pages.AddPage("info", flex, true, true)
}

// symlinkTargetInfoLine returns the "Target:" info-panel line for a symlink
// item, or an empty string when symlink-target display is disabled or the item
// has no symlink target.
func (ui *UI) symlinkTargetInfoLine(item fs.Item) string {
	if !ui.showSymlinkTarget {
		return ""
	}
	si, ok := item.(fs.SymlinkItem)
	if !ok {
		return ""
	}
	target := si.GetSymlinkTarget()
	if target == "" {
		return ""
	}
	return "[::b]Target:[::-] " + tview.Escape(target) + "\n"
}

func (ui *UI) openItem() {
	selectedFile, ok := ui.selectedRowItem()
	if !ok {
		return
	}

	openBinary := "xdg-open"

	switch runtime.GOOS {
	case osDarwin:
		openBinary = "open"
	case osWindows:
		openBinary = "explorer"
	}

	cmd := exec.Command(openBinary, selectedFile.GetPath())
	err := cmd.Start()
	if err != nil {
		ui.showErr("Error opening", err)
	}
}

func (ui *UI) confirmExport() *tview.Form {
	form := tview.NewForm().
		AddInputField("File name", "export.json", 30, nil, func(v string) {
			ui.exportName = v
		}).
		AddButton("Export", ui.exportAnalysis).
		SetButtonsAlign(tview.AlignCenter)
	form.SetBorder(true).
		SetTitle(" Export data to JSON ").
		SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
			if key.Key() == tcell.KeyEsc {
				ui.pages.RemovePage("export")
				ui.app.SetFocus(ui.table)
				return nil
			}
			return key
		})
	flex := modal(form, 50, 7)
	ui.pages.AddPage("export", flex, true, true)
	ui.app.SetFocus(form)
	return form
}

func (ui *UI) exportAnalysis() {
	ui.pages.RemovePage("export")

	// the export format holds exactly one root, so there is nowhere to put the
	// virtual dir grouping several of them
	if analyze.IsVirtualRootDir(ui.session.Root()) {
		ui.showMessage("Export is not supported when multiple directories are scanned")
		return
	}

	text := tview.NewTextView().SetText("Export in progress...").SetTextAlign(tview.AlignCenter)
	text.SetBorder(true).SetTitle(" Export data to JSON ")
	flex := modal(text, 50, 3)
	ui.pages.AddPage("exporting", flex, true, true)

	go func() {
		var err error
		defer ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage("exporting")
			if err == nil {
				ui.app.SetFocus(ui.table)
			}
		})
		if ui.done != nil {
			defer func() {
				ui.done <- struct{}{}
			}()
		}

		var buff bytes.Buffer

		buff.Write([]byte(`[1,2,{"progname":"gdu","progver":"`))
		buff.Write([]byte(build.Version))
		buff.Write([]byte(`","timestamp":`))
		buff.Write([]byte(strconv.FormatInt(time.Now().Unix(), 10)))
		buff.Write([]byte("},\n"))

		file, err := os.Create(ui.exportName)
		if err != nil {
			ui.showErrFromGo("Error creating file", err)
			return
		}
		defer file.Close()

		if err = ui.session.Root().EncodeJSON(&buff, true, nil); err != nil {
			ui.showErrFromGo("Error encoding JSON", err)
			return
		}

		if _, err = buff.Write([]byte("]\n")); err != nil {
			ui.showErrFromGo("Error writing to buffer", err)
			return
		}
		if _, err = buff.WriteTo(file); err != nil {
			ui.showErrFromGo("Error writing to file", err)
			return
		}
	}()
}

func (ui *UI) isInArchive() bool {
	if ui.currentDir == nil {
		return false
	}
	if _, ok := ui.currentDir.(*analyze.ZipDir); ok {
		return true
	}
	_, ok := ui.currentDir.(*analyze.TarDir)
	return ok
}
