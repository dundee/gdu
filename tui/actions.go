package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/report"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const defaultLinesCount = 500
const linesTreshold = 20

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

// AnalyzePath analyzes recursively disk usage for given path
func (ui *UI) AnalyzePath(path string, parentDir fs.Item) error {
	ui.progress = tview.NewTextView().SetText("Scanning...")
	ui.progress.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	ui.progress.SetTitle(" Scanning... ")
	ui.progress.SetDynamicColors(true)

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(ui.progress, 8, 1, false).
			AddItem(nil, 0, 1, false), 0, 50, false).
		AddItem(nil, 0, 1, false)

	ui.pages.AddPage("progress", flex, true, true)
	ui.table.SetSelectedFunc(ui.fileItemSelected)

	go ui.updateProgress()

	go func() {
		defer debug.FreeOSMemory()
		currentDir := ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.ConstGC)

		if parentDir != nil {
			currentDir.SetParent(parentDir)
			parentDir.SetFiles(parentDir.GetFiles().RemoveByName(currentDir.GetName()))
			parentDir.AddFile(currentDir)
		} else {
			ui.topDirPath = path
			ui.topDir = currentDir
		}

		ui.topDir.UpdateStats(ui.linkedItems)

		ui.app.QueueUpdateDraw(func() {
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

func (ui *UI) delete(shouldEmpty bool) {
	if len(ui.markedRows) > 0 {
		ui.deleteMarked(shouldEmpty)
	} else {
		ui.deleteSelected(shouldEmpty)
	}
}

func (ui *UI) deleteSelected(shouldEmpty bool) {
	row, column := ui.table.GetSelection()
	selectedItem := ui.table.GetCell(row, column).GetReference().(fs.Item)

	var action, acting string
	if shouldEmpty {
		action = "empty "
		acting = "emptying"
	} else {
		action = "delete "
		acting = "deleting"
	}
	modal := tview.NewModal().SetText(
		// nolint: staticcheck // Why: fixed string
		strings.Title(acting) +
			" " +
			tview.Escape(selectedItem.GetName()) +
			"...",
	)
	ui.pages.AddPage(acting, modal, true, true)

	var currentDir fs.Item
	var deleteItems []fs.Item
	if shouldEmpty && selectedItem.IsDir() {
		currentDir = selectedItem.(*analyze.Dir)
		for _, file := range currentDir.GetFiles() {
			deleteItems = append(deleteItems, file)
		}
	} else {
		currentDir = ui.currentDir
		deleteItems = append(deleteItems, selectedItem)
	}

	var deleteFun func(fs.Item, fs.Item) error
	if shouldEmpty && !selectedItem.IsDir() {
		deleteFun = ui.emptier
	} else {
		deleteFun = ui.remover
	}
	go func() {
		for _, item := range deleteItems {
			if err := deleteFun(currentDir, item); err != nil {
				msg := "Can't " + action + tview.Escape(selectedItem.GetName())
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
			ui.showDir()
			ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
}

func (ui *UI) showFile() *tview.TextView {
	if ui.currentDir == nil {
		return nil
	}

	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if selectedFile.IsDir() {
		return nil
	}

	f, err := os.Open(selectedFile.GetPath())
	if err != nil {
		ui.showErr("Error opening file", err)
		return nil
	}

	totalLines := 0
	scanner := bufio.NewScanner(f)

	file := tview.NewTextView()
	ui.currentDirLabel.SetText("[::b] --- " +
		strings.TrimPrefix(selectedFile.GetPath(), build.RootPathPrefix) +
		" ---").SetDynamicColors(true)

	readNextPart := func(linesCount int) int {
		var err error
		readLines := 0
		for scanner.Scan() && readLines <= linesCount {
			_, err = file.Write(scanner.Bytes())
			if err != nil {
				ui.showErr("Error reading file", err)
				return 0
			}
			_, err = file.Write([]byte("\n"))
			if err != nil {
				ui.showErr("Error reading file", err)
				return 0
			}
			readLines++
		}
		return readLines
	}
	totalLines += readNextPart(defaultLinesCount)

	file.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyESC {
			err = f.Close()
			if err != nil {
				ui.showErr("Error closing file", err)
				return event
			}
			ui.currentDirLabel.SetText("[::b] --- " +
				strings.TrimPrefix(ui.currentDirPath, build.RootPathPrefix) +
				" ---").SetDynamicColors(true)
			ui.pages.RemovePage("file")
			ui.app.SetFocus(ui.table)
			return event
		}

		switch {
		case event.Rune() == 'j':
			fallthrough
		case event.Rune() == 'G':
			fallthrough
		case event.Key() == tcell.KeyDown:
			fallthrough
		case event.Key() == tcell.KeyPgDn:
			_, _, _, height := file.GetInnerRect()
			row, _ := file.GetScrollOffset()
			if height+row > totalLines-linesTreshold {
				totalLines += readNextPart(defaultLinesCount)
			}
		}
		return event
	})

	grid := tview.NewGrid().SetRows(1, 1, 0, 1).SetColumns(0)
	grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
		AddItem(file, 2, 0, 1, 1, 0, 0, true).
		AddItem(ui.footerLabel, 3, 0, 1, 1, 0, 0, false)

	ui.pages.HidePage("background")
	ui.pages.AddPage("file", grid, true, true)

	return file
}

func (ui *UI) showInfo() {
	if ui.currentDir == nil {
		return
	}

	var content, numberColor string
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(fs.Item)

	if ui.UseColors {
		numberColor = "[#e67100::b]"
	} else {
		numberColor = "[::b]"
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
	content += "[::b]Type:[::-] " + selectedFile.GetType() + "\n\n"

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

func (ui *UI) openItem() {
	row, column := ui.table.GetSelection()
	selectedFile, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedFile == ui.currentDir.GetParent() {
		return
	}

	openBinary := "xdg-open"

	switch runtime.GOOS {
	case "darwin":
		openBinary = "open"
	case "windows":
		openBinary = "explorer"
	}

	cmd := exec.Command(openBinary, selectedFile.GetPath())
	err := cmd.Start()
	if err != nil {
		ui.showErr("Error opening", err)
	}
}
