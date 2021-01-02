package cli

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

const helpText = `
 [red]up, down, k, j    [white]Move cursor up/down
[red]enter, right, l    [white]Select directory/device
        [red]left, h    [white]Go to parent directory
			  [red]d    [white]Delete selected file or directory
			  [red]n    [white]Sort by name (asc/desc)
			  [red]s    [white]Sort by size (asc/desc)
			  [red]c    [white]Sort by items (asc/desc)
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
	topDirPath      string
	currentDirPath  string
	askBeforeDelete bool
	ignorePaths     []string
	sortBy          string
	sortOrder       string
}

// CreateUI creates the whole UI app
func CreateUI(screen tcell.Screen) *UI {
	ui := &UI{
		askBeforeDelete: true,
		sortBy:          "size",
		sortOrder:       "desc",
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
	ui.devices = analyze.GetDevicesInfo()

	ui.table.SetCell(0, 0, tview.NewTableCell("Device name").SetSelectable(false))
	ui.table.SetCell(0, 1, tview.NewTableCell("Size").SetSelectable(false))
	ui.table.SetCell(0, 2, tview.NewTableCell("Used").SetSelectable(false))
	ui.table.SetCell(0, 3, tview.NewTableCell("Used part").SetSelectable(false))
	ui.table.SetCell(0, 4, tview.NewTableCell("Free").SetSelectable(false))
	ui.table.SetCell(0, 5, tview.NewTableCell("Mount point").SetSelectable(false))

	for i, device := range ui.devices {
		ui.table.SetCell(i+1, 0, tview.NewTableCell(device.Name).SetReference(ui.devices[i]))
		ui.table.SetCell(i+1, 1, tview.NewTableCell(formatSize(device.Size)))
		ui.table.SetCell(i+1, 2, tview.NewTableCell(formatSize(device.Size-device.Free)))
		ui.table.SetCell(i+1, 3, tview.NewTableCell(getDeviceUsagePart(device)))
		ui.table.SetCell(i+1, 4, tview.NewTableCell(formatSize(device.Free)))
		ui.table.SetCell(i+1, 5, tview.NewTableCell(device.MountPoint))
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
	ui.progress.SetTitle(" Scanning... ")

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
		ui.currentDir = analyze.ProcessDir(ui.topDirPath, progress, ui.ShouldBeIgnored)

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

// SetIgnorePaths sets paths to ignore
func (ui *UI) SetIgnorePaths(paths []string) {
	ui.ignorePaths = paths
}

// ShouldBeIgnored returns true if given path should be ignored
func (ui *UI) ShouldBeIgnored(path string) bool {
	for _, ignorePath := range ui.ignorePaths {
		if strings.HasPrefix(path, ignorePath) {
			return true
		}
	}
	return false
}

func (ui *UI) showDir() {
	ui.currentDirPath = ui.currentDir.Path
	ui.currentDirLabel.SetText("--- " + ui.currentDirPath + " ---")

	ui.table.Clear()

	rowIndex := 0
	if ui.currentDirPath != ui.topDirPath {
		cell := tview.NewTableCell("           /..")
		cell.SetReference(ui.currentDir.Parent)
		ui.table.SetCell(0, 0, cell)
		rowIndex++
	}

	ui.sortItems()

	for i, item := range ui.currentDir.Files {
		cell := tview.NewTableCell(formatFileRow(item))
		cell.SetReference(ui.currentDir.Files[i])
		ui.table.SetCell(rowIndex, 0, cell)
		rowIndex++
	}

	ui.footer.SetText(
		"Apparent size: " +
			formatSize(ui.currentDir.Size) +
			" Items: " + fmt.Sprint(ui.currentDir.ItemCount) +
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
		if device.Name != selectedDevice.Name {
			ui.ignorePaths = append(ui.ignorePaths, device.MountPoint)
		}
	}

	ui.AnalyzePath(selectedDevice.MountPoint)
}

func (ui *UI) confirmDeletion() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(*analyze.File)
	modal := tview.NewModal().
		SetText("Are you sure you want to delete \"" + selectedFile.Name + "\"").
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
	selectedFile := ui.table.GetCell(row, column).GetReference().(*analyze.File)
	ui.currentDir.RemoveFile(selectedFile)
	ui.showDir()
}

func (ui *UI) keyPressed(key *tcell.EventKey) *tcell.EventKey {
	if (key.Key() == tcell.KeyEsc || key.Rune() == 'q') && ui.pages.HasPage("help") {
		ui.pages.RemovePage("help")
		ui.app.SetFocus(ui.table)
		return key
	}

	if key.Rune() == 'h' || key.Key() == tcell.KeyLeft {
		if ui.currentDirPath == ui.topDirPath {
			return key
		}
		if ui.currentDir != nil {
			ui.fileItemSelected(0, 0)
		} else {
			ui.deviceItemSelected(0, 0)
		}
		return key
	}

	if key.Rune() == 'l' || key.Key() == tcell.KeyRight {
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
		if ui.askBeforeDelete {
			ui.confirmDeletion()
		} else {
			ui.deleteSelected()
		}
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
	for {
		progress.Mutex.Lock()

		if progress.Done {
			return
		}

		ui.app.QueueUpdateDraw(func() {
			ui.progress.SetText("Total items: " +
				fmt.Sprint(progress.ItemCount) +
				" size: " +
				"size: " +
				formatSize(progress.TotalSize) +
				"\nCurrent item: " +
				progress.CurrentItemName)
		})
		progress.Mutex.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}

func (ui *UI) showHelp() {
	text := tview.NewTextView().SetText(helpText).SetDynamicColors(true)
	text.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	text.SetTitle(" gdu help ")

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

func formatFileRow(item *analyze.File) string {
	part := int(float64(item.Size) / float64(item.Parent.Size) * 10.0)
	row := fmt.Sprintf("%10s", formatSize(item.Size))
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
		row += "/"
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
