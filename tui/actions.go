package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const defaultLinesCount = 500
const linesTreshold = 20

// ListDevices lists mounted devices and shows their disk usage
func (ui *UI) ListDevices(getter device.DevicesInfoGetter) error {
	var err error
	ui.devices, err = getter.GetDevicesInfo()
	if err != nil {
		return err
	}

	ui.table.SetCell(0, 0, tview.NewTableCell("Device name").SetSelectable(false))
	ui.table.SetCell(0, 1, tview.NewTableCell("Size").SetSelectable(false))
	ui.table.SetCell(0, 2, tview.NewTableCell("Used").SetSelectable(false))
	ui.table.SetCell(0, 3, tview.NewTableCell("Used part").SetSelectable(false))
	ui.table.SetCell(0, 4, tview.NewTableCell("Free").SetSelectable(false))
	ui.table.SetCell(0, 5, tview.NewTableCell("Mount point").SetSelectable(false))

	var textColor, sizeColor string
	if ui.UseColors {
		textColor = "[#3498db:-:b]"
		sizeColor = "[#edb20a:-:b]"
	} else {
		textColor = "[white:-:b]"
		sizeColor = "[white:-:b]"
	}

	for i, device := range ui.devices {
		ui.table.SetCell(i+1, 0, tview.NewTableCell(textColor+device.Name).SetReference(ui.devices[i]))
		ui.table.SetCell(i+1, 1, tview.NewTableCell(ui.formatSize(device.Size, false, true)))
		ui.table.SetCell(i+1, 2, tview.NewTableCell(sizeColor+ui.formatSize(device.Size-device.Free, false, true)))
		ui.table.SetCell(i+1, 3, tview.NewTableCell(getDeviceUsagePart(device)))
		ui.table.SetCell(i+1, 4, tview.NewTableCell(ui.formatSize(device.Free, false, true)))
		ui.table.SetCell(i+1, 5, tview.NewTableCell(textColor+device.MountPoint))
	}

	ui.table.Select(1, 0)
	ui.footer.SetText("")
	ui.table.SetSelectedFunc(ui.deviceItemSelected)

	return nil
}

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, parentDir *analyze.Dir) error {
	abspath, _ := filepath.Abs(path)

	_, err := ui.PathChecker(abspath)
	if err != nil {
		return err
	}

	ui.progress = tview.NewTextView().SetText("Scanning...")
	ui.progress.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	ui.progress.SetTitle(" Scanning... ")
	ui.progress.SetDynamicColors(true)

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 10, 1, false).
			AddItem(ui.progress, 8, 1, false).
			AddItem(nil, 10, 1, false), 0, 50, false).
		AddItem(nil, 0, 1, false)

	ui.pages.AddPage("progress", flex, true, true)
	ui.table.SetSelectedFunc(ui.fileItemSelected)

	go ui.updateProgress()

	go func() {
		ui.currentDir = ui.Analyzer.AnalyzeDir(abspath, ui.CreateIgnoreFunc())
		runtime.GC()

		if parentDir != nil {
			ui.currentDir.Parent = parentDir
			parentDir.Files = parentDir.Files.RemoveByName(ui.currentDir.Name)
			parentDir.Files.Append(ui.currentDir)

			links := make(analyze.AlreadyCountedHardlinks, 10)
			ui.topDir.UpdateStats(links)
		} else {
			ui.topDirPath = abspath
			ui.topDir = ui.currentDir
		}

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

func (ui *UI) deleteSelected(shouldEmpty bool) {
	row, column := ui.table.GetSelection()
	selectedItem := ui.table.GetCell(row, column).GetReference().(analyze.Item)

	var action, acting string
	if shouldEmpty {
		action = "empty "
		acting = "emptying"
	} else {
		action = "delete "
		acting = "deleting"
	}
	modal := tview.NewModal().SetText(strings.Title(acting) + " " + selectedItem.GetName() + "...")
	ui.pages.AddPage(acting, modal, true, true)

	var currentDir *analyze.Dir
	var deleteItems []analyze.Item
	if shouldEmpty && selectedItem.IsDir() {
		currentDir = selectedItem.(*analyze.Dir)
		for _, file := range currentDir.Files {
			deleteItems = append(deleteItems, file)
		}
	} else {
		currentDir = ui.currentDir
		deleteItems = append(deleteItems, selectedItem)
	}

	var deleteFun func(*analyze.Dir, analyze.Item) error
	if shouldEmpty && !selectedItem.IsDir() {
		deleteFun = ui.emptier
	} else {
		deleteFun = ui.remover
	}
	go func() {
		for _, item := range deleteItems {
			if err := deleteFun(currentDir, item); err != nil {
				msg := "Can't " + action + selectedItem.GetName()
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
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(analyze.Item)
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
	ui.currentDirLabel.SetText("[::b] --- " + selectedFile.GetPath() + " ---").SetDynamicColors(true)

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
			f.Close()
			ui.currentDirLabel.SetText("[::b] --- " + ui.currentDirPath + " ---").SetDynamicColors(true)
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
		AddItem(ui.footer, 3, 0, 1, 1, 0, 0, false)

	ui.pages.HidePage("background")
	ui.pages.AddPage("file", grid, true, true)

	return file
}

func (ui *UI) showInfo() {
	var content, numberColor string
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(analyze.Item)

	if selectedFile == ui.currentDir.Parent {
		return
	}

	if ui.UseColors {
		numberColor = "[#e67100::b]"
	} else {
		numberColor = "[::b]"
	}

	text := tview.NewTextView().SetDynamicColors(true)
	text.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	text.SetBorderColor(tcell.ColorDefault)
	text.SetTitle(" Item info ")

	content += "[::b]Name:[::-] " + selectedFile.GetName() + "\n"
	content += "[::b]Path:[::-] " + selectedFile.GetPath() + "\n"
	content += "[::b]Type:[::-] " + selectedFile.GetType() + "\n\n"

	content += "   [::b]Disk usage:[::-] "
	content += numberColor + ui.formatSize(selectedFile.GetUsage(), false, true)
	content += fmt.Sprintf(" (%s%d[-::] B)", numberColor, selectedFile.GetUsage()) + "\n"
	content += "[::b]Apparent size:[::-] "
	content += numberColor + ui.formatSize(selectedFile.GetSize(), false, true)
	content += fmt.Sprintf(" (%s%d[-::] B)", numberColor, selectedFile.GetSize()) + "\n"

	text.SetText(content)

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, 13, 1, false).
			AddItem(nil, 0, 1, false), 80, 1, false).
		AddItem(nil, 0, 1, false)

	ui.pages.AddPage("info", flex, true, true)
}
