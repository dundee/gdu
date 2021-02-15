package tui

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dundee/gdu/analyze"
	"github.com/dundee/gdu/common"
	"github.com/dundee/gdu/device"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const helpTextColorized = `
 [red]up, down, k, j    [white]Move cursor up/down
[red]enter, right, l    [white]Select directory/device
        [red]left, h    [white]Go to parent directory
			  [red]d    [white]Delete selected file or directory
			  [red]r    [white]Rescan current directory
			  [red]a    [white]Toggle between showing disk usage and apparent size
			  [red]n    [white]Sort by name (asc/desc)
			  [red]s    [white]Sort by size (asc/desc)
			  [red]c    [white]Sort by items (asc/desc)
`
const helpText = `
 [::b]up, down, k, j    [white:black:-]Move cursor up/down
[::b]enter, right, l    [white:black:-]Select directory/device
        [::b]left, h    [white:black:-]Go to parent directory
			  [::b]d    [white:black:-]Delete selected file or directory
			  [::b]r    [white:black:-]Rescan current directory
			  [::b]a    [white:black:-]Toggle between showing disk usage and apparent size
			  [::b]n    [white:black:-]Sort by name (asc/desc)
			  [::b]s    [white:black:-]Sort by size (asc/desc)
			  [::b]c    [white:black:-]Sort by items (asc/desc)
`

// UI struct
type UI struct {
	app              common.Application
	header           *tview.TextView
	footer           *tview.TextView
	currentDirLabel  *tview.TextView
	pages            *tview.Pages
	progress         *tview.TextView
	help             *tview.Flex
	table            *tview.Table
	currentDir       *analyze.File
	devices          []*device.Device
	analyzer         analyze.Analyzer
	topDir           *analyze.File
	topDirPath       string
	currentDirPath   string
	askBeforeDelete  bool
	ignoreDirPaths   map[string]bool
	sortBy           string
	sortOrder        string
	useColors        bool
	showApparentSize bool
}

// CreateUI creates the whole UI app
func CreateUI(app common.Application, useColors bool, showApparentSize bool) *UI {
	ui := &UI{
		askBeforeDelete:  true,
		sortBy:           "size",
		sortOrder:        "desc",
		useColors:        useColors,
		showApparentSize: showApparentSize,
		analyzer:         analyze.ProcessDir,
	}

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Clear()
		return false
	})

	ui.app = app
	ui.app.SetInputCapture(ui.keyPressed)

	ui.header = tview.NewTextView()
	ui.header.SetText(" gdu ~ Use arrow keys to navigate, press ? for help ")
	if ui.useColors {
		ui.header.SetTextColor(tcell.NewRGBColor(0, 0, 0))
		ui.header.SetBackgroundColor(tcell.NewRGBColor(36, 121, 208))
	} else {
		ui.header.SetTextColor(tcell.NewRGBColor(0, 0, 0))
		ui.header.SetBackgroundColor(tcell.NewRGBColor(255, 255, 255))
	}

	ui.currentDirLabel = tview.NewTextView()
	ui.currentDirLabel.SetBackgroundColor(tcell.ColorDefault)

	ui.table = tview.NewTable().SetSelectable(true, false)
	ui.table.SetBackgroundColor(tcell.ColorDefault)
	ui.table.SetSelectedStyle(tcell.Style{}.
		Foreground(tcell.ColorBlack).
		Background(tcell.ColorWhite))

	ui.footer = tview.NewTextView().SetDynamicColors(true)
	if ui.useColors {
		ui.footer.SetTextColor(tcell.NewRGBColor(0, 0, 0))
		ui.footer.SetBackgroundColor(tcell.NewRGBColor(36, 121, 208))
	} else {
		ui.footer.SetTextColor(tcell.NewRGBColor(0, 0, 0))
		ui.footer.SetBackgroundColor(tcell.NewRGBColor(255, 255, 255))
	}
	ui.footer.SetText(" No items to display. ")

	grid := tview.NewGrid().SetRows(1, 1, 0, 1).SetColumns(0)
	grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
		AddItem(ui.table, 2, 0, 1, 1, 0, 0, true).
		AddItem(ui.footer, 3, 0, 1, 1, 0, 0, false)

	ui.pages = tview.NewPages().
		AddPage("background", grid, true, true)
	ui.pages.SetBackgroundColor(tcell.ColorDefault)

	ui.app.SetRoot(ui.pages, true)

	return ui
}

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
func (ui *UI) AnalyzePath(path string, analyzer analyze.Analyzer, parentDir *analyze.File) {
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

	progress := &analyze.CurrentProgress{
		Mutex:     &sync.Mutex{},
		Done:      false,
		ItemCount: 0,
		TotalSize: int64(0),
	}
	go ui.updateProgress(progress)

	go func() {
		ui.currentDir = analyzer(abspath, progress, ui.ShouldDirBeIgnored)

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
	}()
}

// StartUILoop starts tview application
func (ui *UI) StartUILoop() error {
	if err := ui.app.Run(); err != nil {
		return err
	}
	return nil
}

// SetIgnoreDirPaths sets paths to ignore
func (ui *UI) SetIgnoreDirPaths(paths []string) {
	ui.ignoreDirPaths = make(map[string]bool, len(paths))
	for _, path := range paths {
		ui.ignoreDirPaths[path] = true
	}
}

func (ui *UI) rescanDir() {
	ui.AnalyzePath(ui.currentDirPath, ui.analyzer, ui.currentDir.Parent)
}

// ShouldDirBeIgnored returns true if given path should be ignored
func (ui *UI) ShouldDirBeIgnored(path string) bool {
	return ui.ignoreDirPaths[path]
}

func (ui *UI) showDir() {
	ui.currentDirPath = ui.currentDir.Path()
	ui.currentDirLabel.SetText("[::b] --- " + ui.currentDirPath + " ---").SetDynamicColors(true)

	ui.table.Clear()

	rowIndex := 0
	if ui.currentDirPath != ui.topDirPath {
		cell := tview.NewTableCell("                         [::b]/..")
		cell.SetReference(ui.currentDir.Parent)
		ui.table.SetCell(0, 0, cell)
		rowIndex++
	}

	ui.sortItems()

	for i, item := range ui.currentDir.Files {
		cell := tview.NewTableCell(ui.formatFileRow(item))
		cell.SetReference(ui.currentDir.Files[i])

		ui.table.SetCell(rowIndex, 0, cell)
		rowIndex++
	}

	var footerNumberColor, footerTextColor string
	if ui.useColors {
		footerNumberColor = "[#e67100:#2479d0:b]"
		footerTextColor = "[black:#2479d0:-]"
	} else {
		footerNumberColor = "[black:white:b]"
		footerTextColor = "[black:white:-]"
	}

	ui.footer.SetText(
		" Total disk usage: " +
			footerNumberColor +
			ui.formatSize(ui.currentDir.Usage, true) +
			" Apparent size: " +
			footerNumberColor +
			ui.formatSize(ui.currentDir.Size, true) +
			" Items: " + footerNumberColor + fmt.Sprint(ui.currentDir.ItemCount) +
			footerTextColor +
			" Sorting by: " + ui.sortBy + " " + ui.sortOrder)

	ui.table.Select(0, 0)
	ui.table.ScrollToBeginning()
	ui.app.SetFocus(ui.table)
}

func (ui *UI) sortItems() {
	if ui.sortBy == "size" {
		if ui.showApparentSize {
			if ui.sortOrder == "desc" {
				sort.Sort(analyze.ByApparentSize(ui.currentDir.Files))
			} else {
				sort.Sort(sort.Reverse(analyze.ByApparentSize(ui.currentDir.Files)))
			}
		} else {
			if ui.sortOrder == "desc" {
				sort.Sort(ui.currentDir.Files)
			} else {
				sort.Sort(sort.Reverse(ui.currentDir.Files))
			}
		}
	}
	if ui.sortBy == "itemCount" {
		if ui.sortOrder == "desc" {
			sort.Sort(analyze.ByItemCount(ui.currentDir.Files))
		} else {
			sort.Sort(sort.Reverse(analyze.ByItemCount(ui.currentDir.Files)))
		}
	}
	if ui.sortBy == "name" {
		if ui.sortOrder == "desc" {
			sort.Sort(analyze.ByName(ui.currentDir.Files))
		} else {
			sort.Sort(sort.Reverse(analyze.ByName(ui.currentDir.Files)))
		}
	}
}

func (ui *UI) fileItemSelected(row, column int) {
	selectedDir := ui.table.GetCell(row, column).GetReference().(*analyze.File)
	if !selectedDir.IsDir {
		return
	}

	ui.currentDir = selectedDir
	ui.showDir()
}

func (ui *UI) deviceItemSelected(row, column int) {
	selectedDevice := ui.table.GetCell(row, column).GetReference().(*device.Device)

	paths := device.GetNestedMountpointsPaths(selectedDevice.MountPoint, ui.devices)
	for _, path := range paths {
		ui.ignoreDirPaths[path] = true
	}

	ui.AnalyzePath(selectedDevice.MountPoint, ui.analyzer, nil)
}

func (ui *UI) confirmDeletion() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(*analyze.File)
	modal := tview.NewModal().
		SetText("Are you sure you want to delete \"" + selectedFile.Name + "\"").
		AddButtons([]string{"yes", "no", "don't ask me again"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 2:
				ui.askBeforeDelete = false
				fallthrough
			case 0:
				ui.deleteSelected()
			}
			ui.pages.RemovePage("confirm")
		})

	if !ui.useColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	}

	ui.pages.AddPage("confirm", modal, true, true)
}

func (ui *UI) showErr(msg string, err error) {
	modal := tview.NewModal().
		SetText(msg + ": " + err.Error()).
		AddButtons([]string{"ok"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("error")
		})

	if !ui.useColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	}

	ui.pages.AddPage("error", modal, true, true)
}

func (ui *UI) deleteSelected() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(*analyze.File)
	if err := ui.currentDir.RemoveFile(selectedFile); err != nil {
		msg := "Can't delete " + selectedFile.Name
		ui.showErr(msg, err)
		return
	}
	ui.showDir()
	ui.table.Select(min(row, ui.table.GetRowCount()-1), 0)
}

func (ui *UI) keyPressed(key *tcell.EventKey) *tcell.EventKey {
	if (key.Key() == tcell.KeyEsc || key.Rune() == 'q') && ui.pages.HasPage("help") {
		ui.pages.RemovePage("help")
		ui.app.SetFocus(ui.table)
		return key
	}

	if key.Rune() == 'h' || key.Key() == tcell.KeyLeft {
		if ui.pages.HasPage("confirm") {
			return key
		}

		if ui.currentDirPath == ui.topDirPath {
			return key
		}
		if ui.currentDir != nil {
			subDir := ui.currentDir
			ui.fileItemSelected(0, 0)
			index, _ := ui.currentDir.Files.IndexOf(subDir)
			if ui.currentDir != ui.topDir {
				index++
			}
			ui.table.Select(index, 0)
		}
		return key
	}

	if key.Rune() == 'l' || key.Key() == tcell.KeyRight {
		if ui.pages.HasPage("confirm") {
			return key
		}

		row, column := ui.table.GetSelection()
		if ui.currentDirPath != ui.topDirPath && row == 0 { // do not select /..
			return key
		}

		if ui.currentDir != nil {
			ui.fileItemSelected(row, column)
		} else {
			ui.deviceItemSelected(row, column)
		}
		return key
	}

	switch key.Rune() {
	case 'q':
		ui.app.Stop()
		return nil
	case '?':
		ui.showHelp()
		break
	case 'd':
		if ui.currentDir == nil {
			break
		}

		// do not allow deleting parent dir
		row, column := ui.table.GetSelection()
		selectedFile := ui.table.GetCell(row, column).GetReference().(*analyze.File)
		if selectedFile == ui.currentDir.Parent {
			break
		}

		if ui.askBeforeDelete {
			ui.confirmDeletion()
		} else {
			ui.deleteSelected()
		}
		break
	case 'a':
		ui.showApparentSize = !ui.showApparentSize
		if ui.currentDir != nil {
			ui.showDir()
		}
		break
	case 'r':
		if ui.currentDir == nil {
			break
		}
		ui.rescanDir()
		break
	case 's':
		ui.setSorting("size")
		break
	case 'c':
		ui.setSorting("itemCount")
		break
	case 'n':
		ui.setSorting("name")
		break
	}
	return key
}

func (ui *UI) setSorting(newOrder string) {
	if newOrder == ui.sortBy {
		if ui.sortOrder == "asc" {
			ui.sortOrder = "desc"
		} else {
			ui.sortOrder = "asc"
		}
	} else {
		ui.sortBy = newOrder
		ui.sortOrder = "asc"
	}
	ui.showDir()
}

func (ui *UI) updateProgress(progress *analyze.CurrentProgress) {
	color := "[white:-:b]"
	if ui.useColors {
		color = "[red:-:b]"
	}

	for {
		progress.Mutex.Lock()

		if progress.Done {
			return
		}

		ui.app.QueueUpdateDraw(func() {
			ui.progress.SetText("Total items: " +
				color +
				fmt.Sprint(progress.ItemCount) +
				"[white:-:-] size: " +
				color +
				ui.formatSize(progress.TotalSize, false) +
				"[white:-:-]\nCurrent item: [white:-:b]" +
				progress.CurrentItemName)
		})
		progress.Mutex.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}

func (ui *UI) showHelp() {
	text := tview.NewTextView().SetDynamicColors(true)
	text.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	text.SetTitle(" gdu help ")

	if ui.useColors {
		text.SetText(helpTextColorized)
	} else {
		text.SetText(helpText)
	}

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, 15, 1, false).
			AddItem(nil, 0, 1, false), 80, 1, false).
		AddItem(nil, 0, 1, false)

	ui.help = flex
	ui.pages.AddPage("help", flex, true, true)
}

func (ui *UI) formatFileRow(item *analyze.File) string {
	var part int

	if ui.showApparentSize {
		part = int(float64(item.Size) / float64(item.Parent.Size) * 10.0)
	} else {
		part = int(float64(item.Usage) / float64(item.Parent.Usage) * 10.0)
	}

	row := string(item.Flag)

	if ui.useColors {
		row += "[#e67100:-:b]"
	} else {
		row += "[white:-:b]"
	}

	if ui.showApparentSize {
		row += fmt.Sprintf("%21s", ui.formatSize(item.Size, false))
	} else {
		row += fmt.Sprintf("%21s", ui.formatSize(item.Usage, false))
	}

	row += " ["
	for i := 0; i < 10; i++ {
		if part > i {
			row += "#"
		} else {
			row += " "
		}
	}
	row += "] "

	if item.IsDir {
		if ui.useColors {
			row += "[#3498db::b]/"
		} else {
			row += "[::b]/"
		}
	}
	row += item.Name
	return row
}

func (ui *UI) formatSize(size int64, reverseColor bool) string {
	var color string
	if reverseColor {
		if ui.useColors {
			color = "[black:#2479d0:-]"
		} else {
			color = "[black:white:-]"
		}
	} else {
		color = "[white:-:-]"
	}

	switch {
	case size > 1e12:
		return fmt.Sprintf("%.1f%s TiB", float64(size)/math.Pow(2, 40), color)
	case size > 1e9:
		return fmt.Sprintf("%.1f%s GiB", float64(size)/math.Pow(2, 30), color)
	case size > 1e6:
		return fmt.Sprintf("%.1f%s MiB", float64(size)/math.Pow(2, 20), color)
	case size > 1e3:
		return fmt.Sprintf("%.1f%s KiB", float64(size)/math.Pow(2, 10), color)
	default:
		return fmt.Sprintf("%d%s B", size, color)
	}
}

func getDeviceUsagePart(item *device.Device) string {
	part := int(float64(item.Size-item.Free) / float64(item.Size) * 10.0)
	row := "["
	for i := 0; i < 10; i++ {
		if part > i {
			row += "#"
		} else {
			row += " "
		}
	}
	row += "]"
	return row
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
