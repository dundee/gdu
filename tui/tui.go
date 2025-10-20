package tui

import (
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/exp/slices"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/pkg/remove"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI struct
type UI struct {
	app        common.TermApplication
	screen     tcell.Screen
	output     io.Writer
	currentDir fs.Item
	topDir     fs.Item
	getter     device.DevicesInfoGetter
	*common.UI
	grid                    *tview.Grid
	header                  *tview.TextView
	footer                  *tview.Flex
	footerLabel             *tview.TextView
	currentDirLabel         *tview.TextView
	pages                   *tview.Pages
	progress                *tview.TextView
	status                  *tview.TextView
	help                    *tview.Flex
	table                   *tview.Table
	filteringInput          *tview.InputField
	done                    chan struct{}
	remover                 func(fs.Item, fs.Item) error
	emptier                 func(fs.Item, fs.Item) error
	exec                    func(argv0 string, argv []string, envv []string) error
	changeCwdFn             func(string) error
	linkedItems             fs.HardLinkedItems
	ignoredRows             map[int]struct{}
	markedRows              map[int]struct{}
	deleteQueue             chan deleteQueueItem
	resultRow               ResultRow
	topDirPath              string
	currentDirPath          string
	filterValue             string
	sortBy                  string
	sortOrder               string
	footerTextColor         string
	footerBackgroundColor   string
	footerNumberColor       string
	headerTextColor         string
	headerBackgroundColor   string
	defaultSortBy           string
	defaultSortOrder        string
	exportName              string
	devices                 []*device.Device
	selectedTextColor       tcell.Color
	selectedBackgroundColor tcell.Color
	currentItemNameMaxLen   int
	activeWorkers           int
	deleteWorkersCount      int
	statusMut               sync.RWMutex
	workersMut              sync.Mutex
	askBeforeDelete         bool
	showItemCount           bool
	showMtime               bool
	filtering               bool
	headerHidden            bool
	useOldSizeBar           bool
	noDelete                bool
	noSpawnShell            bool
	deleteInBackground      bool
}

type deleteQueueItem struct {
	item        fs.Item
	shouldEmpty bool
}

// ResultRow is a struct for a row in the result table
type ResultRow struct {
	NumberColor    string
	DirectoryColor string
}

// Option is optional function customizing the behaviour of UI
type Option func(ui *UI)

// CreateUI creates the whole UI app
func CreateUI(
	app common.TermApplication,
	screen tcell.Screen,
	output io.Writer,
	useColors bool,
	showApparentSize bool,
	showRelativeSize bool,
	constGC bool,
	useSIPrefix bool,
	opts ...Option,
) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowApparentSize: showApparentSize,
			ShowRelativeSize: showRelativeSize,
			Analyzer:         analyze.CreateAnalyzer(),
			ConstGC:          constGC,
			UseSIPrefix:      useSIPrefix,
		},
		app:                     app,
		screen:                  screen,
		output:                  output,
		askBeforeDelete:         true,
		showItemCount:           false,
		remover:                 remove.ItemFromDir,
		emptier:                 remove.EmptyFileFromDir,
		exec:                    Execute,
		linkedItems:             make(fs.HardLinkedItems, 10),
		selectedTextColor:       tview.Styles.TitleColor,
		selectedBackgroundColor: tview.Styles.MoreContrastBackgroundColor,
		currentItemNameMaxLen:   70,
		defaultSortBy:           "size",
		defaultSortOrder:        "desc",
		ignoredRows:             make(map[int]struct{}),
		markedRows:              make(map[int]struct{}),
		exportName:              "export.json",
		noDelete:                false,
		noSpawnShell:            false,
		deleteQueue:             make(chan deleteQueueItem, 1000),
		deleteWorkersCount:      3 * runtime.GOMAXPROCS(0),
	}
	for _, o := range opts {
		o(ui)
	}

	ui.resetSorting()

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Clear()
		return false
	})

	ui.app.SetInputCapture(ui.keyPressed)
	ui.app.SetMouseCapture(ui.onMouse)

	ui.header = tview.NewTextView()
	ui.header.SetText(" gdu ~ Use arrow keys to navigate, press ? for help ")
	ui.header.SetTextColor(tcell.GetColor(ui.headerTextColor))
	ui.header.SetBackgroundColor(tcell.GetColor(ui.headerBackgroundColor))

	ui.currentDirLabel = tview.NewTextView()
	ui.currentDirLabel.SetTextColor(tcell.ColorDefault)
	ui.currentDirLabel.SetBackgroundColor(tcell.ColorDefault)

	ui.table = tview.NewTable().SetSelectable(true, false)
	ui.table.SetBackgroundColor(tcell.ColorDefault)
	ui.table.SetSelectedFunc(ui.fileItemSelected)

	if ui.UseColors {
		ui.table.SetSelectedStyle(tcell.Style{}.
			Foreground(ui.selectedTextColor).
			Background(ui.selectedBackgroundColor).Bold(true))
	} else {
		ui.table.SetSelectedStyle(tcell.Style{}.
			Foreground(tcell.ColorWhite).
			Background(tcell.ColorGray).Bold(true))
	}

	ui.footerLabel = tview.NewTextView().SetDynamicColors(true)
	ui.footerLabel.SetTextColor(tcell.GetColor(ui.footerTextColor))
	ui.footerLabel.SetBackgroundColor(tcell.GetColor(ui.footerBackgroundColor))
	ui.footerLabel.SetText(" No items to display. ")

	ui.footer = tview.NewFlex()
	ui.footer.AddItem(ui.footerLabel, 0, 1, false)

	ui.createGrid()

	ui.pages = tview.NewPages().
		AddPage("background", ui.grid, true, true)
	ui.pages.SetBackgroundColor(tcell.ColorDefault)

	ui.app.SetRoot(ui.pages, true)

	return ui
}

// createGrid creates the main grid layout
func (ui *UI) createGrid() {
	if ui.headerHidden {
		ui.grid = tview.NewGrid().SetRows(1, 0, 1).SetColumns(0)
		ui.grid.AddItem(ui.currentDirLabel, 0, 0, 1, 1, 0, 0, false).
			AddItem(ui.table, 1, 0, 1, 1, 0, 0, true).
			AddItem(ui.footer, 2, 0, 1, 1, 0, 0, false)
	} else {
		ui.grid = tview.NewGrid().SetRows(1, 1, 0, 1).SetColumns(0)
		ui.grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
			AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
			AddItem(ui.table, 2, 0, 1, 1, 0, 0, true).
			AddItem(ui.footer, 3, 0, 1, 1, 0, 0, false)
	}
}

// SetSelectedTextColor sets the color for the highlighted selected text
func (ui *UI) SetSelectedTextColor(color tcell.Color) {
	ui.selectedTextColor = color
}

// SetSelectedBackgroundColor sets the color for the highlighted selected text
func (ui *UI) SetSelectedBackgroundColor(color tcell.Color) {
	ui.selectedBackgroundColor = color
}

// SetFooterTextColor sets the color for the footer text
func (ui *UI) SetFooterTextColor(color string) {
	ui.footerTextColor = color
}

// SetFooterBackgroundColor sets the color for the footer background
func (ui *UI) SetFooterBackgroundColor(color string) {
	ui.footerBackgroundColor = color
}

// SetFooterNumberColor sets the color for the footer number
func (ui *UI) SetFooterNumberColor(color string) {
	ui.footerNumberColor = color
}

// SetHeaderTextColor sets the color for the header text
func (ui *UI) SetHeaderTextColor(color string) {
	ui.headerTextColor = color
}

// SetHeaderBackgroundColor sets the color for the header background
func (ui *UI) SetHeaderBackgroundColor(color string) {
	ui.headerBackgroundColor = color
}

// SetHeaderHidden sets the flag to hide the header
func (ui *UI) SetHeaderHidden() {
	ui.headerHidden = true
}

// SetResultRowDirectoryColor sets the color for the result row directory
func (ui *UI) SetResultRowDirectoryColor(color string) {
	ui.resultRow.DirectoryColor = color
}

// SetResultRowNumberColor sets the color for the result row number
func (ui *UI) SetResultRowNumberColor(color string) {
	ui.resultRow.NumberColor = color
}

// SetCurrentItemNameMaxLen sets the maximum length of the path of the currently processed item
// to be shown in the progress modal
func (ui *UI) SetCurrentItemNameMaxLen(maxLen int) {
	ui.currentItemNameMaxLen = maxLen
}

// UseOldSizeBar uses the old size bar (# chars) instead of the new one (unicode block elements)
func (ui *UI) UseOldSizeBar() {
	ui.useOldSizeBar = true
}

// SetChangeCwdFn sets function that can be used to change current working dir
// during dir browsing
func (ui *UI) SetChangeCwdFn(fn func(string) error) {
	ui.changeCwdFn = fn
}

// SetDeleteInParallel sets the flag to delete files in parallel
func (ui *UI) SetDeleteInParallel() {
	ui.remover = remove.ItemFromDirParallel
}

// StartUILoop starts tview application
func (ui *UI) StartUILoop() error {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(
			c,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGQUIT,
			syscall.SIGILL,
			syscall.SIGTRAP,
			syscall.SIGABRT,
			syscall.SIGPIPE,
			syscall.SIGTERM,
		)
		s := <-c
		log.Printf("Got signal: %s", s)
		ui.app.QueueUpdateDraw(func() {
			ui.app.Stop()
		})
	}()

	return ui.app.Run()
}

// SetShowItemCount sets the flag to show number of items in directory
func (ui *UI) SetShowItemCount() {
	ui.showItemCount = true
}

// SetShowMTime sets the flag to show last modification time of items in directory
func (ui *UI) SetShowMTime() {
	ui.showMtime = true
}

// SetNoDelete disables all write operations
func (ui *UI) SetNoDelete() {
	ui.noDelete = true
}

// SetNoSpawnShell disables shell spawning
func (ui *UI) SetNoSpawnShell() {
	ui.noSpawnShell = true
}

// SetDeleteInBackground sets the flag to delete files in background
func (ui *UI) SetDeleteInBackground() {
	ui.deleteInBackground = true

	for i := 0; i < ui.deleteWorkersCount; i++ {
		go ui.deleteWorker()
	}
	go ui.updateStatusWorker()
}

func (ui *UI) resetSorting() {
	ui.sortBy = ui.defaultSortBy
	ui.sortOrder = ui.defaultSortOrder
}

func (ui *UI) rescanDir() {
	ui.Analyzer.ResetProgress()
	ui.linkedItems = make(fs.HardLinkedItems)
	err := ui.AnalyzePath(ui.currentDirPath, ui.currentDir.GetParent())
	if err != nil {
		ui.showErr("Error rescanning path", err)
	}
}

func (ui *UI) fileItemSelected(row, column int) {
	if ui.currentDir == nil {
		return // Add this check to handle nil case
	}

	selectedDirCell := ui.table.GetCell(row, column)

	// Check if the selectedDirCell is nil before using it
	if selectedDirCell == nil || selectedDirCell.GetReference() == nil {
		return
	}

	selectedDir := selectedDirCell.GetReference().(fs.Item)
	if selectedDir == nil || !selectedDir.IsDir() {
		return
	}

	origDir := ui.currentDir
	ui.currentDir = selectedDir
	ui.hideFilterInput()
	ui.markedRows = make(map[int]struct{})
	ui.ignoredRows = make(map[int]struct{})
	ui.showDir()

	if origDir.GetParent() != nil && selectedDir.GetName() == origDir.GetParent().GetName() {
		index := slices.IndexFunc(
			ui.currentDir.GetFiles(),
			func(v fs.Item) bool {
				return v.GetName() == origDir.GetName()
			},
		)
		if ui.currentDir.GetPath() != ui.topDir.GetPath() {
			index++
		}
		ui.table.Select(index, 0)
	}
}

func (ui *UI) deviceItemSelected(row, column int) {
	var err error
	selectedDevice, ok := ui.table.GetCell(row, column).GetReference().(*device.Device)
	if !ok {
		return
	}

	paths := device.GetNestedMountpointsPaths(selectedDevice.MountPoint, ui.devices)
	ui.IgnoreDirPathPatterns, err = common.CreateIgnorePattern(paths)
	if err != nil {
		log.Printf("Creating path patterns for other devices failed: %s", paths)
	}

	ui.resetSorting()

	ui.Analyzer.ResetProgress()
	ui.linkedItems = make(fs.HardLinkedItems)
	err = ui.AnalyzePath(selectedDevice.MountPoint, nil)
	if err != nil {
		ui.showErr("Error analyzing device", err)
	}
}

func (ui *UI) confirmDeletion(shouldEmpty bool) {
	if ui.noDelete {
		previousHeaderText := ui.header.GetText(false)

		// show feedback to user
		ui.header.SetText(" Deletion is disabled!")

		go func() {
			time.Sleep(2 * time.Second)
			ui.app.QueueUpdateDraw(func() {
				ui.header.Clear()
				ui.header.SetText(previousHeaderText)
			})
		}()

		return
	}

	if len(ui.markedRows) > 0 {
		ui.confirmDeletionMarked(shouldEmpty)
	} else {
		ui.confirmDeletionSelected(shouldEmpty)
	}
}

func (ui *UI) confirmDeletionSelected(shouldEmpty bool) {
	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(fs.Item)
	var action string
	if shouldEmpty {
		action = "empty"
	} else {
		action = "delete"
	}
	modal := tview.NewModal().
		SetText(
			"Are you sure you want to " +
				action +
				" \"" +
				tview.Escape(selectedFile.GetName()) +
				"\"?",
		).
		AddButtons([]string{"no", "yes", "don't ask me again"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 2:
				ui.askBeforeDelete = false
				fallthrough
			case 1:
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
