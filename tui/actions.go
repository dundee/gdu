package tui

import (
	"path/filepath"

	"github.com/dundee/gdu/analyze"
	"github.com/dundee/gdu/device"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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
	if ui.useColors {
		textColor = "[#3498db:-:b]"
		sizeColor = "[#edb20a:-:b]"
	} else {
		textColor = "[white:-:b]"
		sizeColor = "[white:-:b]"
	}

	for i, device := range ui.devices {
		ui.table.SetCell(i+1, 0, tview.NewTableCell(textColor+device.Name).SetReference(ui.devices[i]))
		ui.table.SetCell(i+1, 1, tview.NewTableCell(ui.formatSize(device.Size, false)))
		ui.table.SetCell(i+1, 2, tview.NewTableCell(sizeColor+ui.formatSize(device.Size-device.Free, false)))
		ui.table.SetCell(i+1, 3, tview.NewTableCell(getDeviceUsagePart(device)))
		ui.table.SetCell(i+1, 4, tview.NewTableCell(ui.formatSize(device.Free, false)))
		ui.table.SetCell(i+1, 5, tview.NewTableCell(textColor+device.MountPoint))
	}

	ui.table.Select(1, 0)
	ui.footer.SetText("")
	ui.table.SetSelectedFunc(ui.deviceItemSelected)

	return nil
}

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, parentDir *analyze.File) {
	abspath, _ := filepath.Abs(path)

	ui.progress = tview.NewTextView().SetText("Scanning...")
	ui.progress.SetBorder(true).SetBorderPadding(2, 2, 2, 2).SetBackgroundColor(tcell.ColorDefault)
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
		ui.currentDir = ui.analyzer.AnalyzeDir(abspath, ui.ShouldDirBeIgnored)

		if parentDir != nil {
			ui.currentDir.Parent = parentDir
			parentDir.Files = parentDir.Files.RemoveByName(ui.currentDir.Name)
			parentDir.Files = append(parentDir.Files, ui.currentDir)

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
}

func (ui *UI) deleteSelected() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(*analyze.File)

	modal := tview.NewModal().SetText("Deleting " + selectedFile.Name + "...")
	ui.pages.AddPage("deleting", modal, true, true)

	currentDir := ui.currentDir

	go func() {
		if err := ui.remover(currentDir, selectedFile); err != nil {
			msg := "Can't delete " + selectedFile.Name
			ui.app.QueueUpdateDraw(func() {
				ui.pages.RemovePage("deleting")
				ui.showErr(msg, err)
			})
			if ui.done != nil {
				ui.done <- struct{}{}
			}
			return
		}

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
