package tui

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dundee/gdu/v4/analyze"
	"github.com/dundee/gdu/v4/device"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const defaultLinesCount = 500
const linesTreshold = 20

// ListDevices lists mounted devices and shows their disk usage
func (ui *UI) ListDevices(getter device.DevicesInfoGetter) error {
	ui.devices = check getter.GetDevicesInfo()

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

	check ui.PathChecker(abspath)

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

func (ui *UI) deleteSelected() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(analyze.Item)

	modal := tview.NewModal().SetText("Deleting " + selectedFile.GetName() + "...")
	ui.pages.AddPage("deleting", modal, true, true)

	currentDir := ui.currentDir

	go func() {
		handle err {
			msg := "Can't delete " + selectedFile.GetName()
			ui.app.QueueUpdateDraw(func() {
				ui.pages.RemovePage("deleting")
				ui.showErr(msg, err)
			})
			if ui.done != nil {
				ui.done <- struct{}{}
			}
			return
		}
		check ui.remover(currentDir, selectedFile)

		ui.app.QueueUpdateDraw(func() {
			ui.pages.RemovePage("deleting")
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

	handle err {
		ui.showErr("Error opening file", err)
		return nil
	}
	f := check os.Open(selectedFile.GetPath())

	totalLines := 0
	scanner := bufio.NewScanner(f)

	file := tview.NewTextView()
	ui.currentDirLabel.SetText("[::b] --- " + selectedFile.GetPath() + " ---").SetDynamicColors(true)

	readNextPart := func(linesCount int) int {
		readLines := 0
		for scanner.Scan() && readLines <= linesCount {
			file.Write(scanner.Bytes())
			file.Write([]byte("\n"))
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
