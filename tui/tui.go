package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const helpText = `    [::b]up/down, k/j    [white:black:-]Move cursor up/down
  [::b]pgup/pgdn, g/G    [white:black:-]Move cursor top/bottom
 [::b]enter, right, l    [white:black:-]Go to directory/device
         [::b]left, h    [white:black:-]Go to parent directory

               [::b]r    [white:black:-]Rescan current directory
               [::b]/    [white:black:-]Search items by name
               [::b]a    [white:black:-]Toggle between showing disk usage and apparent size
               [::b]c    [white:black:-]Show/hide file count
               [::b]q    [white:black:-]Quit gdu

Item under cursor:
               [::b]d    [white:black:-]Delete file or directory
               [::b]e    [white:black:-]Empty file or directory
               [::b]v    [white:black:-]Show content of file
               [::b]i    [white:black:-]Show info about item

Sort by (twice toggles asc/desc):
               [::b]n    [white:black:-]Sort by name (asc/desc)
               [::b]s    [white:black:-]Sort by size (asc/desc)
               [::b]C    [white:black:-]Sort by file count (asc/desc)
`

// UI struct
type UI struct {
	*common.UI
	app             common.TermApplication
	header          *tview.TextView
	footer          *tview.Flex
	footerLabel     *tview.TextView
	currentDirLabel *tview.TextView
	pages           *tview.Pages
	progress        *tview.TextView
	help            *tview.Flex
	table           *tview.Table
	filteringInput  *tview.InputField
	currentDir      *analyze.Dir
	devices         []*device.Device
	topDir          *analyze.Dir
	topDirPath      string
	currentDirPath  string
	askBeforeDelete bool
	showItemCount   bool
	filtering       bool
	filterValue     string
	sortBy          string
	sortOrder       string
	done            chan struct{}
	remover         func(*analyze.Dir, analyze.Item) error
	emptier         func(*analyze.Dir, analyze.Item) error
}

// CreateUI creates the whole UI app
func CreateUI(app common.TermApplication, useColors bool, showApparentSize bool) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowApparentSize: showApparentSize,
			Analyzer:         analyze.CreateAnalyzer(),
		},
		askBeforeDelete: true,
		showItemCount:   false,
		sortBy:          "size",
		sortOrder:       "desc",
		remover:         analyze.RemoveItemFromDir,
		emptier:         analyze.EmptyFileFromDir,
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

	ui.footerLabel = tview.NewTextView().SetDynamicColors(true)
	ui.footerLabel.SetTextColor(textColor)
	ui.footerLabel.SetBackgroundColor(textBgColor)
	ui.footerLabel.SetText(" No items to display. ")

	ui.footer = tview.NewFlex()
	ui.footer.AddItem(ui.footerLabel, 0, 1, false)

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
	return ui.app.Run()
}

func (ui *UI) rescanDir() {
	ui.Analyzer.ResetProgress()
	err := ui.AnalyzePath(ui.currentDirPath, ui.currentDir.Parent)
	if err != nil {
		ui.showErr("Error rescanning path", err)
	}
}

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
			" Items: " + footerNumberColor + fmt.Sprint(itemCount) +
			footerTextColor +
			" Sorting by: " + ui.sortBy + " " + ui.sortOrder)

	ui.table.Select(0, 0)
	ui.table.ScrollToBeginning()

	if !ui.filtering {
		ui.app.SetFocus(ui.table)
	}
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
	ui.hideFilterInput()
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

	err = ui.AnalyzePath(selectedDevice.MountPoint, nil)
	if err != nil {
		ui.showErr("Error analyzing device", err)
	}
}

func (ui *UI) confirmDeletion(shouldEmpty bool) {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(analyze.Item)
	var action string
	if shouldEmpty {
		action = "empty"
	} else {
		action = "delete"
	}
	modal := tview.NewModal().
		SetText("Are you sure you want to " + action + " \"" + selectedFile.GetName() + "\"?").
		AddButtons([]string{"yes", "no", "don't ask me again"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 2:
				ui.askBeforeDelete = false
				fallthrough
			case 0:
				ui.deleteSelected(shouldEmpty)
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
					common.FormatNumber(int64(itemCount)) +
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

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, 27, 1, false).
			AddItem(nil, 0, 1, false), 80, 1, false).
		AddItem(nil, 0, 1, false)

	ui.help = flex
	ui.pages.AddPage("help", flex, true, true)
}
