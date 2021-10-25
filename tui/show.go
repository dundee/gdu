package tui

import (
	"strconv"
	"strings"

	"github.com/dundee/gdu/v5/build"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) showDir() {
	var (
		totalUsage int64
		totalSize  int64
		itemCount  int
	)

	ui.currentDirPath = ui.currentDir.GetPath()
	ui.currentDirLabel.SetText("[::b] --- " +
		strings.TrimPrefix(ui.currentDirPath, build.RootPathPrefix) +
		" ---").SetDynamicColors(true)

	ui.table.Clear()

	rowIndex := 0
	if ui.currentDirPath != ui.topDirPath {
		cell := tview.NewTableCell("                         [::b]/..")
		cell.SetReference(ui.currentDir.Parent)
		cell.SetStyle(tcell.Style{}.Foreground(tcell.ColorDefault))
		ui.table.SetCell(0, 0, cell)
		rowIndex++
	}

	ui.sortItems()

	for i, item := range ui.currentDir.Files {
		if ui.filterValue != "" && !strings.Contains(
			strings.ToLower(item.GetName()),
			strings.ToLower(ui.filterValue),
		) {
			continue
		}

		totalUsage += item.GetUsage()
		totalSize += item.GetSize()
		itemCount += item.GetItemCount()

		cell := tview.NewTableCell(ui.formatFileRow(item))
		cell.SetStyle(tcell.Style{}.Foreground(tcell.ColorDefault))
		cell.SetReference(ui.currentDir.Files[i])

		ui.table.SetCell(rowIndex, 0, cell)
		rowIndex++
	}

	var footerNumberColor, footerTextColor string
	if ui.UseColors {
		footerNumberColor = "[#ffffff:#2479d0:b]"
		footerTextColor = "[black:#2479d0:-]"
	} else {
		footerNumberColor = "[black:white:b]"
		footerTextColor = "[black:white:-]"
	}

	ui.footerLabel.SetText(
		" Total disk usage: " +
			footerNumberColor +
			ui.formatSize(totalUsage, true, false) +
			" Apparent size: " +
			footerNumberColor +
			ui.formatSize(totalSize, true, false) +
			" Items: " + footerNumberColor + strconv.Itoa(itemCount) +
			footerTextColor +
			" Sorting by: " + ui.sortBy + " " + ui.sortOrder)

	ui.table.Select(0, 0)
	ui.table.ScrollToBeginning()

	if !ui.filtering {
		ui.app.SetFocus(ui.table)
	}
}

func (ui *UI) showDevices() {
	var totalUsage int64

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

	ui.sortDevices()

	for i, device := range ui.devices {
		totalUsage += device.GetUsage()
		ui.table.SetCell(i+1, 0, tview.NewTableCell(textColor+device.Name).SetReference(ui.devices[i]))
		ui.table.SetCell(i+1, 1, tview.NewTableCell(ui.formatSize(device.Size, false, true)))
		ui.table.SetCell(i+1, 2, tview.NewTableCell(sizeColor+ui.formatSize(device.Size-device.Free, false, true)))
		ui.table.SetCell(i+1, 3, tview.NewTableCell(getDeviceUsagePart(device)))
		ui.table.SetCell(i+1, 4, tview.NewTableCell(ui.formatSize(device.Free, false, true)))
		ui.table.SetCell(i+1, 5, tview.NewTableCell(textColor+device.MountPoint))
	}

	var footerNumberColor, footerTextColor string
	if ui.UseColors {
		footerNumberColor = "[#ffffff:#2479d0:b]"
		footerTextColor = "[black:#2479d0:-]"
	} else {
		footerNumberColor = "[black:white:b]"
		footerTextColor = "[black:white:-]"
	}

	ui.footerLabel.SetText(
		" Total usage: " +
			footerNumberColor +
			ui.formatSize(totalUsage, true, false) +
			footerTextColor +
			" Sorting by: " + ui.sortBy + " " + ui.sortOrder)

	ui.table.Select(1, 0)
	ui.table.SetSelectedFunc(ui.deviceItemSelected)
}

func (ui *UI) showErr(msg string, err error) {
	modal := tview.NewModal().
		SetText(msg + ": " + err.Error()).
		AddButtons([]string{"ok"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("error")
		})

	if !ui.UseColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	}

	ui.pages.AddPage("error", modal, true, true)
}

func (ui *UI) showHelp() {
	text := tview.NewTextView().SetDynamicColors(true)
	text.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	text.SetBorderColor(tcell.ColorDefault)
	text.SetTitle(" gdu help ")
	text.SetScrollable(true)

	if ui.UseColors {
		text.SetText(
			strings.ReplaceAll(
				strings.ReplaceAll(helpText, "[::b]", "[red]"),
				"[white:black:-]",
				"[white]",
			),
		)
	} else {
		text.SetText(helpText)
	}

	maxHeight := strings.Count(helpText, "\n") + 7
	_, height := ui.screen.Size()
	if height > maxHeight {
		height = maxHeight
	}

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, height, 1, false).
			AddItem(nil, 0, 1, false), 80, 1, false).
		AddItem(nil, 0, 1, false)

	ui.help = flex
	ui.pages.AddPage("help", flex, true, true)
	ui.app.SetFocus(text)
}
