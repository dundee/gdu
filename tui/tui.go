package tui

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dundee/gdu/analyze"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const helpTextColorized = `
 [red]up, down, k, j    [white]Move cursor up/down
[red]enter, right, l    [white]Select directory/device
        [red]left, h    [white]Go to parent directory
			  [red]d    [white]Delete selected file or directory
			  [red]r    [white]Rescan current directory
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
			  [::b]n    [white:black:-]Sort by name (asc/desc)
			  [::b]s    [white:black:-]Sort by size (asc/desc)
			  [::b]c    [white:black:-]Sort by items (asc/desc)
`

// UI struct
type UI struct {
	app             *tview.Application
	header          *tview.TextView
	footer          *tview.TextView
	currentDirLabel *tview.TextView
	pages           *tview.Pages
	progress        *tview.TextView
	help            *tview.Flex
	table           *tview.Table
	currentDir      *analyze.File
	devices         []*analyze.Device
	analyzer        analyze.Analyzer
	topDir          *analyze.File
	topDirPath      string
	currentDirPath  string
	askBeforeDelete bool
	ignoreDirPaths  map[string]bool
	sortBy          string
	sortOrder       string
	useColors       bool
}

// CreateUI creates the whole UI app
func CreateUI(screen tcell.Screen, useColors bool) *UI {
	ui := &UI{
		askBeforeDelete: true,
		sortBy:          "size",
		sortOrder:       "desc",
		useColors:       useColors,
		analyzer:        analyze.ProcessDir,
	}

	ui.app = tview.NewApplication()
	ui.app.SetScreen(screen)
	ui.app.SetInputCapture(ui.keyPressed)

	ui.header = tview.NewTextView()
	ui.header.SetText(" gdu ~ Use arrow keys to navigate, press ? for help ")
	if ui.useColors {
		ui.header.SetTextColor(tcell.ColorWhite)
		ui.header.SetBackgroundColor(tcell.NewRGBColor(36, 121, 208))
	} else {
		ui.header.SetTextColor(tcell.ColorBlack)
		ui.header.SetBackgroundColor(tcell.ColorWhite)
	}

	ui.currentDirLabel = tview.NewTextView()

	ui.table = tview.NewTable().SetSelectable(true, false)

	if ui.useColors {
		ui.table.SetSelectedStyle(tcell.Style{}.
			Foreground(tcell.ColorBlack).
			Background(tcell.NewRGBColor(52, 152, 219)))
	}

	ui.footer = tview.NewTextView().SetDynamicColors(true)
	if ui.useColors {
		ui.footer.SetTextColor(tcell.ColorWhite)
		ui.footer.SetBackgroundColor(tcell.NewRGBColor(36, 121, 208))
	} else {
		ui.footer.SetTextColor(tcell.ColorBlack)
		ui.footer.SetBackgroundColor(tcell.ColorWhite)
	}
	ui.footer.SetText(" No items to diplay. ")

	grid := tview.NewGrid().SetRows(1, 1, 0, 1).SetColumns(0)
	grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
		AddItem(ui.table, 2, 0, 1, 1, 0, 0, true).
		AddItem(ui.footer, 3, 0, 1, 1, 0, 0, false)

	ui.pages = tview.NewPages().
		AddPage("background", grid, true, true)

	ui.app.SetRoot(ui.pages, true)

	return ui
}

// ListDevices lists mounted devices and shows their disk usage
func (ui *UI) ListDevices() {
	var err error
	ui.devices, err = analyze.GetDevicesInfo("/proc/mounts")
	if err != nil {
		panic(err)
	}

	ui.table.SetCell(0, 0, tview.NewTableCell("Device name").SetSelectable(false))
	ui.table.SetCell(0, 1, tview.NewTableCell("Size").SetSelectable(false))
	ui.table.SetCell(0, 2, tview.NewTableCell("Used").SetSelectable(false))
	ui.table.SetCell(0, 3, tview.NewTableCell("Used part").SetSelectable(false))
	ui.table.SetCell(0, 4, tview.NewTableCell("Free").SetSelectable(false))
	ui.table.SetCell(0, 5, tview.NewTableCell("Mount point").SetSelectable(false))

	var textColor, sizeColor string
	if ui.useColors {
		textColor = "[#3498db:black:b]"
		sizeColor = "[#edb20a:black:b]"
	} else {
		textColor = "[white:black:b]"
		sizeColor = "[white:black:b]"
	}

	for i, device := range ui.devices {
		ui.table.SetCell(i+1, 0, tview.NewTableCell(textColor+device.Name).SetReference(ui.devices[i]))
		ui.table.SetCell(i+1, 1, tview.NewTableCell(formatSize(device.Size, false, ui.useColors)))
		ui.table.SetCell(i+1, 2, tview.NewTableCell(sizeColor+formatSize(device.Size-device.Free, false, ui.useColors)))
		ui.table.SetCell(i+1, 3, tview.NewTableCell(getDeviceUsagePart(device)))
		ui.table.SetCell(i+1, 4, tview.NewTableCell(formatSize(device.Free, false, ui.useColors)))
		ui.table.SetCell(i+1, 5, tview.NewTableCell(textColor+device.MountPoint))
	}

	ui.table.Select(1, 0)
	ui.footer.SetText("")
	ui.table.SetSelectedFunc(ui.deviceItemSelected)
}

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string, analyzer analyze.Analyzer, parentDir *analyze.File) {
	abspath, _ := filepath.Abs(path)

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
			ui.topDir.UpdateStats()
		} else {
			ui.topDirPath = abspath
			ui.topDir = ui.currentDir
		}

		ui.app.QueueUpdateDraw(func() {
			ui.showDir()
			ui.pages.HidePage("progress")
		})
	}()
}

// StartUILoop starts tview application
func (ui *UI) StartUILoop() {
	if err := ui.app.Run(); err != nil {
		panic(err)
	}
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
		cell := tview.NewTableCell("                        [::b]/..")
		cell.SetReference(ui.currentDir.Parent)
		ui.table.SetCell(0, 0, cell)
		rowIndex++
	}

	ui.sortItems()

	for i, item := range ui.currentDir.Files {
		cell := tview.NewTableCell(formatFileRow(item, ui.useColors))
		cell.SetReference(ui.currentDir.Files[i])

		ui.table.SetCell(rowIndex, 0, cell)
		rowIndex++
	}

	var footerNumberColor, footerTextColor string
	if ui.useColors {
		footerNumberColor = "[#e74c3c:#2479d0:b]"
		footerTextColor = "[white:#2479d0:-]"
	} else {
		footerNumberColor = "[black:white:b]"
		footerTextColor = "[black:white:-]"
	}

	ui.footer.SetText(
		" Apparent size: " +
			footerNumberColor +
			formatSize(ui.currentDir.Size, true, ui.useColors) +
			" Items: " + footerNumberColor + fmt.Sprint(ui.currentDir.ItemCount) +
			footerTextColor +
			" Sorting by: " + ui.sortBy + " " + ui.sortOrder)
	ui.table.Select(0, 0)
	ui.table.ScrollToBeginning()
	ui.app.SetFocus(ui.table)
}

func (ui *UI) sortItems() {
	if ui.sortBy == "size" {
		if ui.sortOrder == "desc" {
			sort.Sort(ui.currentDir.Files)
		} else {
			sort.Sort(sort.Reverse(ui.currentDir.Files))
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
	selectedDevice := ui.table.GetCell(row, column).GetReference().(*analyze.Device)

	for _, device := range ui.devices {
		if device.Name != selectedDevice.Name && !strings.HasPrefix(selectedDevice.MountPoint, device.MountPoint) {
			ui.ignoreDirPaths[device.MountPoint] = true
		}
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
			if buttonIndex == 0 || buttonIndex == 2 {
				ui.deleteSelected()
			}
			if buttonIndex == 1 {
				ui.pages.RemovePage("confirm")
				return
			} else if buttonIndex == 2 {
				ui.askBeforeDelete = false
			}
			ui.pages.RemovePage("confirm")
		})

	if !ui.useColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	}

	ui.pages.AddPage("confirm", modal, true, true)
}

func (ui *UI) deleteSelected() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(*analyze.File)
	if err := ui.currentDir.RemoveFile(selectedFile); err != nil {
		panic(err)
	}
	ui.showDir()
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
			index := ui.currentDir.Files.Find(subDir)
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
		if ui.askBeforeDelete {
			ui.confirmDeletion()
		} else {
			ui.deleteSelected()
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
	color := "[white:black:b]"
	if ui.useColors {
		color = "[red:black:b]"
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
				"[white:black:-] size: " +
				color +
				formatSize(progress.TotalSize, false, ui.useColors) +
				"[white:black:-]\nCurrent item: [white:black:b]" +
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
			AddItem(nil, 0, 1, false), 60, 1, false).
		AddItem(nil, 0, 1, false)

	ui.help = flex
	ui.pages.AddPage("help", flex, true, true)
}

func formatSize(size int64, reverseColor bool, useColors bool) string {
	var color string
	if reverseColor {
		if useColors {
			color = "[white:#2479d0:-]"
		} else {
			color = "[black:white:-]"
		}
	} else {
		color = "[white:black:-]"
	}

	if size > 1e12 {
		return fmt.Sprintf("%.1f%s TiB", float64(size)/math.Pow(2, 40), color)
	} else if size > 1e9 {
		return fmt.Sprintf("%.1f%s GiB", float64(size)/math.Pow(2, 30), color)
	} else if size > 1e6 {
		return fmt.Sprintf("%.1f%s MiB", float64(size)/math.Pow(2, 20), color)
	} else if size > 1e3 {
		return fmt.Sprintf("%.1f%s KiB", float64(size)/math.Pow(2, 10), color)
	}
	return fmt.Sprintf("%d%s B", size, color)
}

func formatFileRow(item *analyze.File, useColor bool) string {
	part := int(float64(item.Size) / float64(item.Parent.Size) * 10.0)
	var row string

	if useColor {
		row = "[#edb20a:black:b]"
	} else {
		row = "[white:black:b]"
	}

	row += fmt.Sprintf("%25s", formatSize(item.Size, false, useColor))
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
		if useColor {
			row += "[#3498db::b]/"
		} else {
			row += "[::b]/"
		}
	}
	row += item.Name
	return row
}

func getDeviceUsagePart(item *analyze.Device) string {
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
