package tui

import (
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/dundee/gdu/v4/analyze"
	"github.com/dundee/gdu/v4/common"
	"github.com/dundee/gdu/v4/device"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const helpTextColorized = `
  [red]up, down, k, j    [white]Move cursor up/down
[red]pgup, pgdn, g, G    [white]Move cursor top/bottom
 [red]enter, right, l    [white]Select directory/device
         [red]left, h    [white]Go to parent directory
			   [red]d    [white]Delete selected file or directory
			   [red]v    [white]Show content of selected file
			   [red]r    [white]Rescan current directory
			   [red]a    [white]Toggle between showing disk usage and apparent size
			   [red]n    [white]Sort by name (asc/desc)
			   [red]s    [white]Sort by size (asc/desc)
			   [red]c    [white]Sort by items (asc/desc)
			   [red]q    [white]Quit gdu
`
const helpText = `
  [::b]up, down, k, j    [white:black:-]Move cursor up/down
[::b]pgup, pgdn, g, G    [white:black:-]Move cursor top/bottom
 [::b]enter, right, l    [white:black:-]Select directory/device
         [::b]left, h    [white:black:-]Go to parent directory
			   [::b]d    [white:black:-]Delete selected file or directory
			   [::b]v    [white:black:-]Show content of selected file
			   [::b]r    [white:black:-]Rescan current directory
			   [::b]a    [white:black:-]Toggle between showing disk usage and apparent size
			   [::b]n    [white:black:-]Sort by name (asc/desc)
			   [::b]s    [white:black:-]Sort by size (asc/desc)
			   [::b]c    [white:black:-]Sort by items (asc/desc)
			   [::b]q    [white:black:-]Quit gdu
`

// UI struct
type UI struct {
	*common.UI
	app             common.TermApplication
	header          *tview.TextView
	footer          *tview.TextView
	currentDirLabel *tview.TextView
	pages           *tview.Pages
	progress        *tview.TextView
	help            *tview.Flex
	table           *tview.Table
	currentDir      *analyze.Dir
	devices         []*device.Device
	topDir          *analyze.Dir
	topDirPath      string
	currentDirPath  string
	askBeforeDelete bool
	sortBy          string
	sortOrder       string
	done            chan struct{}
	remover         func(*analyze.Dir, analyze.Item) error
}

// CreateUI creates the whole UI app
func CreateUI(app common.TermApplication, useColors bool, showApparentSize bool) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowApparentSize: showApparentSize,
			Analyzer:         analyze.CreateAnalyzer(),
			PathChecker:      os.Stat,
		},
		askBeforeDelete: true,
		sortBy:          "size",
		sortOrder:       "desc",
		remover:         analyze.RemoveItemFromDir,
	}

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Clear()
		return false
	})

	ui.app = app
	ui.app.SetInputCapture(ui.keyPressed)

	var textColor, textBgColor tcell.Color
	if ui.UseColors {
		textColor = tcell.NewRGBColor(0, 0, 0)
		textBgColor = tcell.NewRGBColor(36, 121, 208)
	} else {
		textColor = tcell.NewRGBColor(0, 0, 0)
		textBgColor = tcell.NewRGBColor(255, 255, 255)
	}

	ui.header = tview.NewTextView()
	ui.header.SetText(" gdu ~ Use arrow keys to navigate, press ? for help ")
	ui.header.SetTextColor(textColor)
	ui.header.SetBackgroundColor(textBgColor)

	ui.currentDirLabel = tview.NewTextView()
	ui.currentDirLabel.SetTextColor(tcell.ColorDefault)
	ui.currentDirLabel.SetBackgroundColor(tcell.ColorDefault)

	ui.table = tview.NewTable().SetSelectable(true, false)
	ui.table.SetBackgroundColor(tcell.ColorDefault)

	if ui.UseColors {
		ui.table.SetSelectedStyle(tcell.Style{}.
			Foreground(tview.Styles.TitleColor).
			Background(tview.Styles.MoreContrastBackgroundColor).Bold(true))
	} else {
		ui.table.SetSelectedStyle(tcell.Style{}.
			Foreground(tcell.ColorWhite).
			Background(tcell.ColorGray).Bold(true))
	}

	ui.footer = tview.NewTextView().SetDynamicColors(true)
	ui.footer.SetTextColor(textColor)
	ui.footer.SetBackgroundColor(textBgColor)
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

// StartUILoop starts tview application
func (ui *UI) StartUILoop() error {
	check ui.app.Run()
	return nil
}

func (ui *UI) rescanDir() {
	ui.Analyzer.ResetProgress()
	ui.AnalyzePath(ui.currentDirPath, ui.currentDir.Parent)
}

func (ui *UI) showDir() {
	ui.currentDirPath = ui.currentDir.GetPath()
	ui.currentDirLabel.SetText("[::b] --- " + ui.currentDirPath + " ---").SetDynamicColors(true)

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

	ui.footer.SetText(
		" Total disk usage: " +
			footerNumberColor +
			ui.formatSize(ui.currentDir.Usage, true, false) +
			" Apparent size: " +
			footerNumberColor +
			ui.formatSize(ui.currentDir.Size, true, false) +
			" Items: " + footerNumberColor + fmt.Sprint(ui.currentDir.ItemCount) +
			footerTextColor +
			" Sorting by: " + ui.sortBy + " " + ui.sortOrder)

	ui.table.Select(0, 0)
	ui.table.ScrollToBeginning()
	ui.app.SetFocus(ui.table)
}

func (ui *UI) sortItems() {
	if ui.sortBy == "size" {
		if ui.ShowApparentSize {
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
	origDir := ui.currentDir
	selectedDir := ui.table.GetCell(row, column).GetReference().(analyze.Item)
	if !selectedDir.IsDir() {
		return
	}

	ui.currentDir = selectedDir.(*analyze.Dir)
	ui.showDir()

	if selectedDir == origDir.Parent {
		index, _ := ui.currentDir.Files.IndexOf(origDir)
		if ui.currentDir != ui.topDir {
			index++
		}
		ui.table.Select(index, 0)
	}
}

func (ui *UI) deviceItemSelected(row, column int) {
	var err error
	selectedDevice := ui.table.GetCell(row, column).GetReference().(*device.Device)

	paths := device.GetNestedMountpointsPaths(selectedDevice.MountPoint, ui.devices)
	ui.IgnoreDirPathPatterns, err = common.CreateIgnorePattern(paths)

	if err != nil {
		log.Printf("Creating path patterns for other devices failed: %s", paths)
	}

	ui.AnalyzePath(selectedDevice.MountPoint, nil)
}

func (ui *UI) confirmDeletion() {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(analyze.Item)
	modal := tview.NewModal().
		SetText("Are you sure you want to delete \"" + selectedFile.GetName() + "\"?").
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

	if !ui.UseColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	} else {
		modal.SetBackgroundColor(tcell.ColorBlack)
	}
	modal.SetBorderColor(tcell.ColorDefault)

	ui.pages.AddPage("confirm", modal, true, true)
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

func (ui *UI) updateProgress() {
	color := "[white:black:b]"
	if ui.UseColors {
		color = "[red:black:b]"
	}

	progressChan := ui.Analyzer.GetProgressChan()
	doneChan := ui.Analyzer.GetDoneChan()

	var progress analyze.CurrentProgress

	for {
		select {
		case progress = <-progressChan:
		case <-doneChan:
			return
		}

		func(itemCount int, totalSize int64, currentItem string) {
			ui.app.QueueUpdateDraw(func() {
				ui.progress.SetText("Total items: " +
					color +
					fmt.Sprint(itemCount) +
					"[white:black:-] size: " +
					color +
					ui.formatSize(totalSize, false, false) +
					"[white:black:-]\nCurrent item: [white:black:b]" +
					currentItem)
			})
		}(progress.ItemCount, progress.TotalSize, progress.CurrentItemName)

		time.Sleep(100 * time.Millisecond)
	}
}

func (ui *UI) showHelp() {
	text := tview.NewTextView().SetDynamicColors(true)
	text.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	text.SetBorderColor(tcell.ColorDefault)
	text.SetTitle(" gdu help ")

	if ui.UseColors {
		text.SetText(helpTextColorized)
	} else {
		text.SetText(helpText)
	}

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, 20, 1, false).
			AddItem(nil, 0, 1, false), 80, 1, false).
		AddItem(nil, 0, 1, false)

	ui.help = flex
	ui.pages.AddPage("help", flex, true, true)
}
