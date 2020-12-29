package main

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const helpText = `
[red]up, down [white]Move cursor
       [red]d [white]Delete selected file or dir
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
	currentDir      *File
	topDirPath      string
	currentDirPath  string
	askBeforeDelete bool
}

// CreateUI creates the whole UI app
func CreateUI(screen tcell.Screen) *UI {
	ui := &UI{
		askBeforeDelete: true,
	}

	ui.app = tview.NewApplication()
	ui.app.SetScreen(screen)
	ui.app.SetInputCapture(ui.keyPressed)

	ui.header = tview.NewTextView()
	ui.header.SetText("gdu ~ Use arrow keys to navigate, press ? for help")
	ui.header.SetTextColor(tcell.ColorBlack)
	ui.header.SetBackgroundColor(tcell.ColorWhite)

	ui.currentDirLabel = tview.NewTextView()

	ui.table = tview.NewTable().SetSelectable(true, false)

	ui.footer = tview.NewTextView()
	ui.footer.SetTextColor(tcell.ColorBlack)
	ui.footer.SetBackgroundColor(tcell.ColorWhite)
	ui.footer.SetText("No items to diplay.")

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
	devices := GetDevicesInfo()

	ui.table.SetCell(0, 0, tview.NewTableCell("Device name").SetSelectable(false))
	ui.table.SetCell(0, 1, tview.NewTableCell("Size").SetSelectable(false))
	ui.table.SetCell(0, 2, tview.NewTableCell("Used").SetSelectable(false))
	ui.table.SetCell(0, 3, tview.NewTableCell("Used part").SetSelectable(false))
	ui.table.SetCell(0, 4, tview.NewTableCell("Free").SetSelectable(false))
	ui.table.SetCell(0, 5, tview.NewTableCell("Mount point").SetSelectable(false))

	for i, device := range devices {
		ui.table.SetCell(i+1, 0, tview.NewTableCell(device.name).SetReference(devices[i]))
		ui.table.SetCell(i+1, 1, tview.NewTableCell(formatSize(device.size)))
		ui.table.SetCell(i+1, 2, tview.NewTableCell(formatSize(device.size-device.free)))
		ui.table.SetCell(i+1, 3, tview.NewTableCell(getDeviceUsagePart(device)))
		ui.table.SetCell(i+1, 4, tview.NewTableCell(formatSize(device.free)))
		ui.table.SetCell(i+1, 5, tview.NewTableCell(device.mountPoint))
	}

	ui.table.Select(1, 0)
	ui.footer.SetText("")
	ui.table.SetSelectedFunc(ui.deviceItemSelected)
}

// AnalyzePath analyzes recursively disk usage in given path
func (ui *UI) AnalyzePath(path string) {
	ui.topDirPath, _ = filepath.Abs(path)

	ui.progress = tview.NewTextView().SetText("Scanning...")
	ui.progress.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	ui.progress.SetTitle("Scanning...")

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 10, 1, false).
			AddItem(ui.progress, 8, 1, false).
			AddItem(nil, 10, 1, false), 0, 50, false).
		AddItem(nil, 0, 1, false)

	ui.pages.AddPage("progress", flex, true, true)
	ui.table.SetSelectedFunc(ui.fileItemSelected)

	progress := &CurrentProgress{
		mutex:     &sync.Mutex{},
		done:      false,
		itemCount: 0,
		totalSize: int64(0),
	}
	go ui.updateProgress(progress)

	go func() {
		ui.currentDir = ProcessDir(ui.topDirPath, progress, ui.ShouldBeIgnored)

		ui.app.QueueUpdateDraw(func() {
			ui.showDir()
			ui.pages.HidePage("progress")
		})
	}()
}

// showDir shows content of the selected dir
func (ui *UI) showDir() {
	ui.currentDirPath = ui.currentDir.path
	ui.currentDirLabel.SetText("--- " + ui.currentDirPath + " ---")

	ui.table.Clear()

	rowIndex := 0
	if ui.currentDirPath != ui.topDirPath {
		cell := tview.NewTableCell("           /..")
		cell.SetReference(ui.currentDir.parent)
		ui.table.SetCell(0, 0, cell)
		rowIndex++
	}

	sort.Slice(ui.currentDir.files, func(i, j int) bool {
		return ui.currentDir.files[i].size > ui.currentDir.files[j].size
	})

	for i, item := range ui.currentDir.files {
		cell := tview.NewTableCell(formatFileRow(item))
		cell.SetReference(ui.currentDir.files[i])
		ui.table.SetCell(rowIndex, 0, cell)
		rowIndex++
	}

	ui.footer.SetText("Apparent size: " + formatSize(ui.currentDir.size) + " Items: " + fmt.Sprint(ui.currentDir.itemCount))
	ui.table.Select(0, 0)
	ui.table.ScrollToBeginning()
	ui.app.SetFocus(ui.table)
}

// StartUILoop starts tview application
func (ui *UI) StartUILoop() {
	if err := ui.app.Run(); err != nil {
		panic(err)
	}
}

// ShouldBeIgnored returns true if given path should be ignored
func (ui *UI) ShouldBeIgnored(path string) bool {
	if strings.HasPrefix(path, "/proc") || strings.HasPrefix(path, "/dev") || strings.HasPrefix(path, "/sys") || strings.HasPrefix(path, "/run") {
		return true
	}
	return false
}

func (ui *UI) fileItemSelected(row, column int) {
	selectedDir := ui.table.GetCell(row, column).GetReference().(*File)
	if !selectedDir.isDir {
		return
	}

	ui.currentDir = selectedDir
	ui.showDir()
}

func (ui *UI) deviceItemSelected(row, column int) {
	selectedDevice := ui.table.GetCell(row, column).GetReference().(*Device)
	ui.AnalyzePath(selectedDevice.mountPoint)
}

func (ui *UI) confirmDeletion() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(*File)
	modal := tview.NewModal().
		SetText("Are you sure you want to delete \"" + selectedFile.name + "\"").
		AddButtons([]string{"yes", "no", "don't ask me again"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonIndex == 1 {
				ui.pages.HidePage("confirm")
				return
			} else if buttonIndex == 2 {
				ui.askBeforeDelete = false
			}
			ui.pages.HidePage("confirm")
			ui.deleteSelected()
		})
	ui.pages.AddPage("confirm", modal, true, true)
}

func (ui *UI) deleteSelected() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(*File)
	ui.currentDir.RemoveFile(selectedFile)
	ui.showDir()
}

func (ui *UI) keyPressed(key *tcell.EventKey) *tcell.EventKey {
	if (key.Key() == tcell.KeyEsc || key.Rune() == 'q') && ui.pages.HasPage("help") {
		ui.pages.RemovePage("help")
		ui.app.SetFocus(ui.table)
		return key
	}
	if key.Rune() == 'q' {
		ui.app.Stop()
		return nil
	}
	if key.Rune() == '?' {
		ui.showHelp()
	}
	if key.Rune() == 'd' {
		if ui.askBeforeDelete {
			ui.confirmDeletion()
		} else {
			ui.deleteSelected()
		}
	}
	return key
}

func (ui *UI) updateProgress(progress *CurrentProgress) {
	for {
		progress.mutex.Lock()

		if progress.done {
			return
		}

		ui.app.QueueUpdateDraw(func() {
			ui.progress.SetText("Total items: " +
				fmt.Sprint(progress.itemCount) +
				" size: " +
				"size: " +
				formatSize(progress.totalSize) +
				"\nCurrent item: " +
				progress.currentItemName)
		})
		progress.mutex.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}

func (ui *UI) showHelp() {
	text := tview.NewTextView().SetText(helpText).SetDynamicColors(true)
	text.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	text.SetTitle("gdu help")

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, 10, 1, false).
			AddItem(nil, 0, 1, false), 50, 1, false).
		AddItem(nil, 0, 1, false)

	ui.help = flex
	ui.pages.AddPage("help", flex, true, true)
}

func formatSize(size int64) string {
	if size > 1e12 {
		return fmt.Sprintf("%.1f TiB", float64(size)/math.Pow(2, 40))
	} else if size > 1e9 {
		return fmt.Sprintf("%.1f GiB", float64(size)/math.Pow(2, 30))
	} else if size > 1e6 {
		return fmt.Sprintf("%.1f MiB", float64(size)/math.Pow(2, 20))
	} else if size > 1e3 {
		return fmt.Sprintf("%.1f KiB", float64(size)/math.Pow(2, 10))
	}
	return fmt.Sprintf("%d B", size)
}

func formatFileRow(item *File) string {
	part := int(float64(item.size) / float64(item.parent.size) * 10.0)
	row := fmt.Sprintf("%10s", formatSize(item.size))
	row += " ["
	for i := 0; i < 10; i++ {
		if part > i {
			row += "#"
		} else {
			row += " "
		}
	}
	row += "] "

	if item.isDir {
		row += "/"
	}
	row += item.name
	return row
}

func getDeviceUsagePart(item *Device) string {
	part := int(float64(item.size-item.free) / float64(item.size) * 10.0)
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
