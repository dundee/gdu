package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/report"
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
func (ui *UI) AnalyzePath(path string, parentDir fs.Item) error {
	ui.showScanProgressModal(" Scanning... ")

	ui.scanCancelRequested.Store(false)
	ui.Analyzer.ResetProgress()

	analyzer := ui.Analyzer
	doneChan := analyzer.GetDone()
	go ui.updateProgress(analyzer, doneChan)

	ui.scanStart = time.Now()
	ui.scanning = true
	ui.scanCancelled = false
	go func() {
		defer debug.FreeOSMemory()
		currentDir := analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter())

		if parentDir != nil {
			currentDir.SetParent(parentDir)
			// Remove old entry with the same name and add new one
			parentDir.RemoveFileByName(currentDir.GetName())
			parentDir.AddFile(currentDir)
		} else {
			ui.topDirPath = path
			ui.topDir = currentDir
		}

		if ui.IsFilteringFiles() {
			ui.topDir.UpdateStatsWithFileFiltering(ui.linkedItems)
		} else {
			ui.topDir.UpdateStats(ui.linkedItems)
		}

		ui.app.QueueUpdateDraw(func() {
			ui.scanning = false
			ui.scanCancelled = false
			// remember the longest scan so we can guard against accidental quits
			if d := time.Since(ui.scanStart); d > ui.scanDuration {
				ui.scanDuration = d
			}
			// the finished scan replaces any mid-scan preview
			ui.previewing = false
			ui.previewSavedDir = nil
			ui.currentDir = currentDir
			ui.showDir()
			ui.pages.RemovePage("progress")
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()

	return nil
}

// AnalyzePaths scans several roots one after another and presents them under a
// virtual top level dir, so that the rest of the UI keeps working on the single
// rooted tree it expects.
//
// The roots are scanned sequentially rather than concurrently because the
// analyzer owns a single set of progress channels; each root needs the analyzer
// reset before it starts, which is also why cancellation has to be tracked
// separately (ResetProgress clears the analyzer's own cancelled flag).
func (ui *UI) AnalyzePaths(paths []string) error {
	if len(paths) == 1 {
		return ui.AnalyzePath(paths[0], nil)
	}

	ui.showScanProgressModal(scanProgressTitle(1, len(paths)))

	ui.scanCancelRequested.Store(false)
	ui.scanStart = time.Now()
	ui.scanning = true
	ui.scanCancelled = false

	go func() {
		defer debug.FreeOSMemory()

		roots := make([]fs.Item, 0, len(paths))
		for i, path := range paths {
			if i > 0 {
				title := scanProgressTitle(i+1, len(paths))
				ui.app.QueueUpdateDraw(func() {
					ui.progress.SetTitle(title)
				})
			}

			ui.Analyzer.ResetProgress()
			analyzer := ui.Analyzer
			go ui.updateProgress(analyzer, analyzer.GetDone())

			roots = append(
				roots,
				analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter()),
			)

			// keep whatever was scanned so far, as a cancelled single scan does
			if ui.scanCancelRequested.Load() {
				break
			}
		}

		currentDir := analyze.CreateVirtualRootDir(roots...)
		ui.topDir = currentDir
		ui.topDirPath = currentDir.GetPath()

		if ui.IsFilteringFiles() {
			ui.topDir.UpdateStatsWithFileFiltering(ui.linkedItems)
		} else {
			ui.topDir.UpdateStats(ui.linkedItems)
		}

		ui.app.QueueUpdateDraw(func() {
			ui.scanning = false
			ui.scanCancelled = false
			// remember the longest scan so we can guard against accidental quits
			if d := time.Since(ui.scanStart); d > ui.scanDuration {
				ui.scanDuration = d
			}
			// the finished scan replaces any mid-scan preview
			ui.previewing = false
			ui.previewSavedDir = nil
			ui.currentDir = currentDir
			ui.showDir()
			ui.pages.RemovePage("progress")
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()

	return nil
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
		var err error
		ui.currentDir, err = report.ReadAnalysis(input)
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
		runtime.GC()

		ui.topDirPath = ui.currentDir.GetPath()
		ui.topDir = ui.currentDir

		links := make(fs.HardLinkedItems, 10)
		ui.topDir.UpdateStats(links)

		ui.app.QueueUpdateDraw(func() {
			ui.showDir()
			ui.pages.RemovePage("progress")
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()

	return nil
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

	ui.currentDir = dir
	ui.topDirPath = ui.currentDir.GetPath()
	ui.topDir = ui.currentDir

	ui.showDir()
	return nil
}

func (ui *UI) delete(action DeleteAction) {
	if len(ui.markedRows) > 0 {
		ui.deleteMarked(action)
	} else {
		ui.deleteSelected(action)
	}
}

func (ui *UI) deleteSelected(action DeleteAction) {
	row, column := ui.table.GetSelection()
	selectedItem := ui.table.GetCell(row, column).GetReference().(fs.Item)

	if ui.deleteInBackground {
		ui.queueForDeletion([]fs.Item{selectedItem}, action)
		return
	}

	acting := action.Acting()
	modal := tview.NewModal().SetText(
		cases.Title(language.English).String(acting) +
			" " +
			tview.Escape(selectedItem.GetName()) +
			"...",
	)
	ui.pages.AddPage(acting, modal, true, true)

	var currentDir fs.Item
	var deleteItems []fs.Item
	if action == ActionEmpty && selectedItem.IsDir() {
		currentDir = selectedItem
		for file := range currentDir.GetFiles(fs.SortBySize, fs.SortDesc) {
			deleteItems = append(deleteItems, file)
		}
	} else {
		currentDir = ui.currentDir
		deleteItems = append(deleteItems, selectedItem)
	}

	var deleteFun func(fs.Item, fs.Item) error
	switch action {
	case ActionEmpty:
		if !selectedItem.IsDir() {
			deleteFun = ui.emptier
		} else {
			deleteFun = ui.remover
		}
	case ActionMoveToTrash:
		deleteFun = ui.trasher
	case ActionDelete:
		deleteFun = ui.remover
	}
	go func() {
		for _, item := range deleteItems {
			if err := deleteFun(currentDir, item); err != nil {
				msg := "Can't " + action.Verb() + " " + tview.Escape(selectedItem.GetName())
				ui.app.QueueUpdateDraw(func() {
					ui.pages.RemovePage(acting)
					ui.showErr(msg, err)
				})
				if ui.done != nil {
					ui.done <- struct{}{}
				}
				return
			}
		}

		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage(acting)
			x, y := ui.table.GetOffset()
			ui.showDir()
			ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
			ui.table.SetOffset(min(x, ui.table.GetRowCount()-1), y)
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
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
		linkedItems := ui.linkedItems[selectedFile.GetMultiLinkedInode()]
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
	row, column := ui.table.GetSelection()
	selectedFile, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedFile == ui.currentDir.GetParent() {
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
	if analyze.IsVirtualRootDir(ui.topDir) {
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

		if err = ui.topDir.EncodeJSON(&buff, true, nil); err != nil {
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
