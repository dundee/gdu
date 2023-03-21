package tui

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/build"
)

const helpText = `     [::b]up/down, k/j    [white:black:-]Move cursor up/down
  [::b]pgup/pgdn, g/G     [white:black:-]Move cursor top/bottom
 [::b]enter, right, l     [white:black:-]Go to directory/device
         [::b]left, h     [white:black:-]Go to parent directory

               [::b]r     [white:black:-]Rescan current directory
               [::b]/     [white:black:-]Search items by name
               [::b]a     [white:black:-]Toggle between showing disk usage and apparent size
               [::b]B     [white:black:-]Toggle bar alignment to biggest file or directory
               [::b]c     [white:black:-]Show/hide file count
               [::b]m     [white:black:-]Show/hide latest mtime
               [::b]b     [white:black:-]Spawn shell in current directory
               [::b]q     [white:black:-]Quit gdu
               [::b]Q     [white:black:-]Quit gdu and print current directory path

Item under cursor:
               [::b]d     [white:black:-]Delete file or directory
               [::b]e     [white:black:-]Empty file or directory
			   [::b]space [white:black:-]Mark file or directory for deletion
               [::b]v     [white:black:-]Show content of file
               [::b]o     [white:black:-]Open file or directory in external program
               [::b]i     [white:black:-]Show info about item

Sort by (twice toggles asc/desc):
               [::b]n     [white:black:-]Sort by name (asc/desc)
               [::b]s     [white:black:-]Sort by size (asc/desc)
               [::b]C     [white:black:-]Sort by file count (asc/desc)
               [::b]M     [white:black:-]Sort by mtime (asc/desc)`

func (ui *UI) showDir() {
	var (
		totalUsage int64
		totalSize  int64
		maxUsage   int64
		maxSize    int64
		itemCount  int
	)

	ui.currentDirPath = ui.currentDir.GetPath()

	if ui.changeCwdFn != nil {
		err := ui.changeCwdFn(ui.currentDirPath)
		if err != nil {
			log.Printf("error setting cwd: %s", err.Error())
		}
		log.Printf("changing cwd to %s", ui.currentDirPath)
	}

	ui.currentDirLabel.SetText("[::b] --- " +
		tview.Escape(
			strings.TrimPrefix(ui.currentDirPath, build.RootPathPrefix),
		) +
		" ---").SetDynamicColors(true)

	ui.table.Clear()

	rowIndex := 0
	if ui.currentDirPath != ui.topDirPath {
		prefix := "                         "
		if len(ui.markedRows) > 0 {
			prefix += "  "
		}

		cell := tview.NewTableCell(prefix + "[::b]/..")
		cell.SetReference(ui.currentDir.GetParent())
		cell.SetStyle(tcell.Style{}.Foreground(tcell.ColorDefault))
		ui.table.SetCell(0, 0, cell)
		rowIndex++
	}

	ui.sortItems()

	if ui.ShowRelativeSize {
		for _, item := range ui.currentDir.GetFiles() {
			if item.GetUsage() > maxUsage {
				maxUsage = item.GetUsage()
			}
			if item.GetSize() > maxSize {
				maxSize = item.GetSize()
			}
		}
	} else {
		maxUsage = ui.currentDir.GetUsage()
		maxSize = ui.currentDir.GetSize()
	}

	for i, item := range ui.currentDir.GetFiles() {
		if ui.filterValue != "" && !strings.Contains(
			strings.ToLower(item.GetName()),
			strings.ToLower(ui.filterValue),
		) {
			continue
		}

		totalUsage += item.GetUsage()
		totalSize += item.GetSize()
		itemCount += item.GetItemCount()

		_, marked := ui.markedRows[rowIndex]
		cell := tview.NewTableCell(ui.formatFileRow(item, maxUsage, maxSize, marked))
		cell.SetReference(ui.currentDir.GetFiles()[i])

		if marked {
			cell.SetStyle(tcell.Style{}.Foreground(tview.Styles.PrimaryTextColor))
			cell.SetBackgroundColor(tview.Styles.ContrastBackgroundColor)
		} else {
			cell.SetStyle(tcell.Style{}.Foreground(tcell.ColorDefault))
		}

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

	selected := ""
	if len(ui.markedRows) > 0 {
		selected = " Selected items: " + footerNumberColor +
			strconv.Itoa(len(ui.markedRows)) + footerTextColor
	}

	ui.footerLabel.SetText(
		selected +
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

	ui.table.Clear()
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

	if ui.topDirPath != "" {
		for i, device := range ui.devices {
			if device.MountPoint == ui.topDirPath {
				ui.table.Select(i+1, 0)
				break
			}
		}
	}
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
