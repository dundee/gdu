package tui

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/dundee/gdu/v6/internal/common"
	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/remove"
	"github.com/dundee/gdu/v6/internal/session"
	"github.com/dundee/gdu/v6/internal/timefilter"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI struct
type UI struct {
	app        TermApplication
	screen     tcell.Screen
	output     io.Writer
	currentDir fs.Item
	session    *session.Session
	getter     device.DevicesInfoGetter
	*common.UI
	grid                    *tview.Grid
	header                  *tview.TextView
	footer                  *tview.Flex
	footerLabel             *tview.TextView
	currentDirLabel         *tview.TextView
	pages                   *tview.Pages
	progress                *tview.TextView
	progressBar             *ProgressBar
	status                  *tview.TextView
	help                    *tview.Flex
	table                   *tview.Table
	filteringInput          *tview.InputField
	typeFilteringInput      *tview.InputField
	done                    chan struct{}
	trasher                 remove.Trash
	newDeleter              func(roots []string) itemDeleter
	exec                    func(argv0 string, argv []string, envv []string) error
	changeCwdFn             func(string) error
	ignoredRows             map[int]struct{}
	markedItems             map[fs.Item]struct{}
	markedPaths             []string
	parentRowShown          bool
	deleteInParallel        bool
	deleteQueue             chan deleteQueueItem
	resultRow               ResultRow
	topDirPath              string
	currentDirPath          string
	filterValue             string
	typeFilterValue         string
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
	markedTextColor         tcell.Color
	markedBackgroundColor   tcell.Color
	currentItemNameMaxLen   int
	activeWorkers           int
	// pendingDeletions counts the items queued or being deleted, and
	// quitAfterDeletions quits once there are none; both guarded by
	// workersMut
	pendingDeletions    int
	quitAfterDeletions  func()
	deleteWorkersCount  int
	statusMut           sync.RWMutex
	workersMut          sync.Mutex
	askBeforeDelete     bool
	showItemCount       bool
	showSymlinkTarget   bool
	showMtime           bool
	filtering           bool
	typeFiltering       bool
	headerHidden        bool
	useOldSizeBar       bool
	showBarPercentage   bool
	showItemCountBar    bool
	noDelete            bool
	noViewFile          bool
	noSpawnShell        bool
	shellBin            string
	deleteInBackground  bool
	timeFilter          *timefilter.TimeFilter
	timeFilterLoc       *time.Location
	noDeleteWithFilter  bool
	collapsePath        bool
	browseParentDirs    bool
	showDiskProgressBar bool
	currentDeviceSize   int64
	confirmQuit         bool
	ctrlCQuits          bool
	scanning            bool
	scanCancelled       bool
	// stopScan cancels the running scan
	stopScan        context.CancelFunc
	scanStart       time.Time
	scanDuration    time.Duration
	previewing      bool
	previewSavedDir fs.Item
	progressFlex    *tview.Flex
}

type deleteQueueItem struct {
	target  deleteTarget
	action  DeleteAction
	deleter itemDeleter
}

// ResultRow is a struct for a row in the result table
type ResultRow struct {
	NumberColor    string
	DirectoryColor string
}

// CreateUI returns the terminal UI for the settings s, running in app on
// screen and scanning with analyzer. output receives what is printed on quit.
func CreateUI(app TermApplication, screen tcell.Screen, output io.Writer, analyzer analyze.Analyzer, s *config.Settings) *UI {
	ui := &UI{
		UI:                      common.NewUI(analyzer, s),
		app:                     app,
		screen:                  screen,
		output:                  output,
		askBeforeDelete:         true,
		confirmQuit:             true,
		showItemCount:           false,
		trasher:                 remove.MoveItemToTrash,
		exec:                    Execute,
		selectedTextColor:       tview.Styles.TitleColor,
		selectedBackgroundColor: tview.Styles.MoreContrastBackgroundColor,
		markedTextColor:         tview.Styles.PrimaryTextColor,
		markedBackgroundColor:   tview.Styles.ContrastBackgroundColor,
		currentItemNameMaxLen:   70,
		defaultSortBy:           "size",
		defaultSortOrder:        "desc",
		ignoredRows:             make(map[int]struct{}),
		markedItems:             make(map[fs.Item]struct{}),
		exportName:              "export.json",
		noDelete:                false,
		noViewFile:              false,
		noSpawnShell:            false,
		deleteQueue:             make(chan deleteQueueItem, 1000),
		deleteWorkersCount:      3 * runtime.GOMAXPROCS(0),
	}
	ui.newDeleter = ui.defaultDeleter
	ui.session = session.New(ui.UI)
	if !s.Display.SIPrefix {
		ui.SetBlockSizeFromEnvironment()
	}
	ui.apply(s)

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
	setStyles(s.Display.Colors)

	return ui
}

// setStyles sets the colours tview draws its widgets with, which are global
// to tview.
func setStyles(colors bool) {
	if colors {
		tview.Styles.TitleColor = tcell.NewRGBColor(27, 161, 227)
	} else {
		tview.Styles.ContrastBackgroundColor = tcell.NewRGBColor(150, 150, 150)
	}
	tview.Styles.BorderColor = tcell.ColorDefault
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

// apply configures the UI with the settings s.
//
//nolint:gocyclo // a suite of if statements
func (ui *UI) apply(s *config.Settings) {
	style := s.TUI.Style
	for _, c := range []struct {
		value string
		set   func(string)
	}{
		{style.SelectedRowText, func(c string) { ui.setSelectedTextColor(tcell.GetColor(c)) }},
		{style.SelectedRowBackground, func(c string) { ui.setSelectedBackgroundColor(tcell.GetColor(c)) }},
		{style.MarkedText, func(c string) { ui.setMarkedTextColor(tcell.GetColor(c)) }},
		{style.MarkedBackground, func(c string) { ui.setMarkedBackgroundColor(tcell.GetColor(c)) }},
		{style.FooterText, ui.setFooterTextColor},
		{style.FooterBackground, ui.setFooterBackgroundColor},
		{style.FooterNumber, ui.setFooterNumberColor},
		{style.HeaderText, ui.setHeaderTextColor},
		{style.HeaderBackground, ui.setHeaderBackgroundColor},
		{style.ResultRowNumber, ui.setResultRowNumberColor},
		{style.ResultRowDirectory, ui.setResultRowDirectoryColor},
	} {
		if c.value != "" {
			c.set(c.value)
		}
	}
	if style.HeaderHidden {
		ui.setHeaderHidden()
	}
	if style.CurrentItemNameMaxLen > 0 {
		ui.setCurrentItemNameMaxLen(style.CurrentItemNameMaxLen)
	}
	ui.useOldSizeBar = style.OldSizeBar
	ui.showBarPercentage = style.ShowBarPercentage
	ui.showItemCountBar = style.ShowItemCountBar
	ui.showDiskProgressBar = style.ShowDiskProgressBar

	ui.setDefaultSorting(s.Display.SortBy, s.Display.SortOrder)
	ui.showItemCount = s.Display.ItemCount
	ui.showMtime = s.Display.MTime
	ui.showSymlinkTarget = s.Display.SymlinkTarget
	ui.collapsePath = s.Display.CollapsePath

	if s.TUI.ChangeCwd {
		ui.setChangeCwdFn(os.Chdir)
	}
	ui.noDelete = s.Delete.Disabled
	ui.noViewFile = s.TUI.NoViewFile
	ui.noSpawnShell = s.TUI.NoSpawnShell
	ui.confirmQuit = !s.TUI.NoConfirmQuit
	ui.ctrlCQuits = s.TUI.CtrlCQuits
	ui.deleteInParallel = s.Delete.InParallel
	if s.Delete.TrashCommand != "" {
		ui.setTrashCommand(s.Delete.TrashCommand)
	}
	ui.shellBin = s.TUI.Shell
	ui.browseParentDirs = s.TUI.BrowseParentDirs
	ui.setTimeFilterWithInfo(s.Scan.TimeFilter, s.Scan.TimeLocation)
	if s.Delete.InBackground {
		ui.setDeleteInBackground()
	}
}

// setSelectedTextColor sets the color for the highlighted selected text
func (ui *UI) setSelectedTextColor(color tcell.Color) {
	ui.selectedTextColor = color
}

// setSelectedBackgroundColor sets the color for the highlighted selected text
func (ui *UI) setSelectedBackgroundColor(color tcell.Color) {
	ui.selectedBackgroundColor = color
}

// setMarkedTextColor sets the text color for marked items
func (ui *UI) setMarkedTextColor(color tcell.Color) {
	ui.markedTextColor = color
}

// setMarkedBackgroundColor sets the background color for marked items
func (ui *UI) setMarkedBackgroundColor(color tcell.Color) {
	ui.markedBackgroundColor = color
}

// setFooterTextColor sets the color for the footer text
func (ui *UI) setFooterTextColor(color string) {
	ui.footerTextColor = color
}

// setFooterBackgroundColor sets the color for the footer background
func (ui *UI) setFooterBackgroundColor(color string) {
	ui.footerBackgroundColor = color
}

// setFooterNumberColor sets the color for the footer number
func (ui *UI) setFooterNumberColor(color string) {
	ui.footerNumberColor = color
}

// setHeaderTextColor sets the color for the header text
func (ui *UI) setHeaderTextColor(color string) {
	ui.headerTextColor = color
}

// setHeaderBackgroundColor sets the color for the header background
func (ui *UI) setHeaderBackgroundColor(color string) {
	ui.headerBackgroundColor = color
}

// setHeaderHidden sets the flag to hide the header
func (ui *UI) setHeaderHidden() {
	ui.headerHidden = true
}

// setResultRowDirectoryColor sets the color for the result row directory
func (ui *UI) setResultRowDirectoryColor(color string) {
	ui.resultRow.DirectoryColor = color
}

// setResultRowNumberColor sets the color for the result row number
func (ui *UI) setResultRowNumberColor(color string) {
	ui.resultRow.NumberColor = color
}

// setCurrentItemNameMaxLen sets the maximum length of the path of the currently processed item
// to be shown in the progress modal
func (ui *UI) setCurrentItemNameMaxLen(maxLen int) {
	ui.currentItemNameMaxLen = maxLen
}

// setOldSizeBar uses the old size bar (# chars) instead of the new one (unicode block elements)
func (ui *UI) setOldSizeBar() {
	ui.useOldSizeBar = true
}

// setShowBarPercentage shows the numeric usage percentage next to the size bar
func (ui *UI) setShowBarPercentage() {
	ui.showBarPercentage = true
}

// setShowItemCountBar shows a bar next to the item count column, drawn in
// proportion to the item counts of the sibling items. It only takes effect
// while the item count column itself is visible (the "c" key).
func (ui *UI) setShowItemCountBar() {
	ui.showItemCountBar = true
}

// setChangeCwdFn sets function that can be used to change current working dir
// during dir browsing
func (ui *UI) setChangeCwdFn(fn func(string) error) {
	ui.changeCwdFn = fn
}

// setDeleteInParallel sets the flag to delete files in parallel
func (ui *UI) setDeleteInParallel() {
	ui.deleteInParallel = true
}

// setTrashCommand replaces the built-in trash with an external command
func (ui *UI) setTrashCommand(command string) {
	ui.trasher = remove.TrashCommand(command)
}

// StartUILoop starts tview application
func (ui *UI) StartUILoop() error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, handledSignals...)
	defer signal.Stop(signals)

	return ui.runUILoop(signals)
}

func (ui *UI) runUILoop(signals <-chan os.Signal) error {
	stopSignals := make(chan struct{})
	signalsDone := make(chan struct{})
	go func() {
		defer close(signalsDone)
		ui.handleSignals(signals, stopSignals)
	}()

	err := ui.app.Run()
	close(stopSignals)
	<-signalsDone
	return err
}

func (ui *UI) handleSignals(signals <-chan os.Signal, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case current, ok := <-signals:
			if !ok {
				return
			}
			slog.Info("got signal", "signal", current)
			ui.postSignalEvent(signalEvent(current), stop)
		}
	}
}

func (ui *UI) postSignalEvent(event *tcell.EventKey, stop <-chan struct{}) {
	if err := ui.screen.PostEvent(event); err == nil {
		return
	}

	retry := time.NewTicker(time.Millisecond)
	defer retry.Stop()
	for {
		select {
		case <-stop:
			return
		case <-retry.C:
			if err := ui.screen.PostEvent(event); err == nil {
				return
			}
		}
	}
}

func signalEvent(current os.Signal) *tcell.EventKey {
	modifiers := tcell.ModNone
	if current == syscall.SIGINT {
		modifiers = tcell.ModCtrl
	}
	return tcell.NewEventKey(tcell.KeyExit, 0, modifiers)
}

func (ui *UI) handleSignalEvent(event *tcell.EventKey) {
	// A second Ctrl+C while the scan is stopping falls through to quitting.
	if event.Modifiers() == tcell.ModCtrl && !ui.ctrlCQuits && ui.cancelScan() {
		return
	}
	ui.quitNow()
}

// quitNow shuts the app down straight away, without asking for confirmation.
func (ui *UI) quitNow() {
	ui.printMarkedPaths()
	ui.app.Stop()
}

func (ui *UI) cancelScan() bool {
	if !ui.scanning || ui.scanCancelled {
		return false
	}

	if ui.stopScan != nil {
		ui.stopScan()
	}
	ui.scanCancelled = true
	ui.progress.SetTitle(" Stopping scan... ")
	stopping := "Stopping scan and keeping results..."
	if !ui.ctrlCQuits {
		stopping += "\n\nPress Ctrl+C again to quit gdu"
	}
	ui.progress.SetText(stopping)
	return true
}

// setConfirmQuit sets whether gdu asks for confirmation before quitting
// after a scan that took a noticeable amount of time
func (ui *UI) setConfirmQuit(value bool) {
	ui.confirmQuit = value
}

// setCtrlCQuits makes Ctrl+C quit gdu during a scan instead of stopping the
// scan and keeping the results found so far. Esc always stops the scan,
// regardless of this setting.
func (ui *UI) setCtrlCQuits() {
	ui.ctrlCQuits = true
}

// setShowItemCount sets the flag to show number of items in directory
func (ui *UI) setShowItemCount() {
	ui.showItemCount = true
}

// setShowMTime sets the flag to show last modification time of items in directory
func (ui *UI) setShowMTime() {
	ui.showMtime = true
}

// setShowSymlinkTarget enables displaying the symlink target (name -> target)
func (ui *UI) setShowSymlinkTarget(value bool) {
	ui.showSymlinkTarget = value
}

// setNoDelete disables all write operations
func (ui *UI) setNoDelete() {
	ui.noDelete = true
}

// setNoSpawnShell disables shell spawning
func (ui *UI) setNoSpawnShell() {
	ui.noSpawnShell = true
}

// setShellBin sets the shell binary spawned by the b key.
func (ui *UI) setShellBin(path string) {
	ui.shellBin = path
}

func (ui *UI) setNoViewFile() {
	ui.noViewFile = true
}

// setNoDelete disables delete when time filters are active
func (ui *UI) setNoDeleteWithFilter() {
	ui.noDeleteWithFilter = true
}

// setBrowseParentDirs enables navigating above the launch directory
func (ui *UI) setBrowseParentDirs() {
	ui.browseParentDirs = true
}

// setCollapsePath sets the flag to collapse paths
func (ui *UI) setCollapsePath(value bool) {
	ui.collapsePath = value
}

// setShowDiskProgressBar sets whether to show a progress bar when scanning a whole disk
func (ui *UI) setShowDiskProgressBar(value bool) {
	ui.showDiskProgressBar = value
}

// setDeleteInBackground sets the flag to delete files in background
func (ui *UI) setDeleteInBackground() {
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

// atVirtualRoot reports whether the view is at the synthetic dir grouping
// several scanned roots. That dir has no path on disk, so operations that
// resolve currentDirPath back to the filesystem must not run there.
func (ui *UI) atVirtualRoot() bool {
	return analyze.IsVirtualRootDir(ui.currentDir)
}

func (ui *UI) rescanDir() {
	ui.rescan(ui.currentDir)
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
	ui.hideTypeFilterInput()
	ui.markedItems = make(map[fs.Item]struct{})
	ui.ignoredRows = make(map[int]struct{})
	ui.showDir()

	// while previewing a mid-scan snapshot there is no stable top dir to anchor
	// the "select last visited" logic to, so just render the navigated dir
	if ui.previewing {
		return
	}

	if row != 0 || origDir.GetPath() == ui.topDirPath {
		return
	}

	// we are going up in the directory tree, select the last visited directory
	if origDir.GetParent() != nil {
		nestedDir := origDir
		for nestedDir.GetParent() != nil {
			if selectedDir.GetName() == nestedDir.GetParent().GetName() {
				index := ui.indexOf(nestedDir.GetName())
				if index >= 0 {
					if ui.currentDir.GetPath() != ui.topDirPath {
						index++
					}
					ui.table.Select(index, 0)
				}
				break
			}
			nestedDir = nestedDir.GetParent()
		}
	}
}

// indexOf returns the position of the entry of the current directory named
// name, as listed, or -1.
func (ui *UI) indexOf(name string) int {
	defer ui.readTree()()
	sortBy, sortOrder := ui.getSortParams()
	i := 0
	for item := range ui.currentDir.GetFiles(sortBy, sortOrder) {
		if item.GetName() == name {
			return i
		}
		i++
	}
	return -1
}

// readTree holds the lock of the session's tree for reading until the
// returned function is called. Whoever holds it must not take it again.
func (ui *UI) readTree() func() {
	lock := ui.session.TreeLock()
	lock.RLock()
	return lock.RUnlock
}

func (ui *UI) deviceItemSelected(row, column int) {
	var err error
	selectedDevice, ok := ui.table.GetCell(row, column).GetReference().(*device.Device)
	if !ok {
		return
	}

	// every mount, not only the devices listed: tmpfs, overlay, network
	// and FUSE filesystems below the device are not part of it either
	mounts, err := ui.getter.GetMounts()
	if err != nil {
		slog.Warn("listing mounts", "err", err)
		mounts = ui.devices
	}
	ui.IgnoreOtherDevices(device.GetNestedMountpointsPaths(selectedDevice.MountPoint, mounts))

	ui.resetSorting()

	ui.currentDeviceSize = selectedDevice.Size
	err = ui.AnalyzePath(selectedDevice.MountPoint, nil)
	if err != nil {
		ui.showErr("Error analyzing device", err)
	}
}

// confirmDeletion asks before deleting targets, which were captured when the
// deletion was requested: answering yes deletes exactly those, whatever is
// selected or listed by then.
func (ui *UI) confirmDeletion(targets []deleteTarget, action DeleteAction) {
	if ui.noDelete {
		ui.showMessage(" Deletion is disabled!")
		return
	}

	// Check if deletion is allowed with active time filters
	if ui.noDeleteWithFilter {
		modal := tview.NewModal().
			SetText("Deletion is disabled when a time filter is active.\n\n" +
				"To override, set GDU_ALLOW_DELETE_WITH_FILTER=1").
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				ui.pages.RemovePage("confirm")
			})
		if !ui.UseColors {
			modal.SetBackgroundColor(tcell.ColorGray)
		}
		ui.pages.AddPage("confirm", modal, true, true)
		return
	}

	if len(ui.markedItems) > 0 {
		ui.confirmDeletionMarked(targets, action)
	} else {
		ui.confirmDeletionSelected(targets, action)
	}
}

func (ui *UI) confirmDeletionSelected(targets []deleteTarget, action DeleteAction) {
	if len(targets) == 0 {
		return
	}
	modal := tview.NewModal().
		SetText(
			"Are you sure you want to " +
				action.Verb() +
				" \"" +
				tview.Escape(targets[0].item.GetName()) +
				"\"?",
		).
		AddButtons([]string{No, Yes, DontAskAgain}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonIndex {
			case 2:
				ui.askBeforeDelete = false
				fallthrough
			case 1:
				ui.deleteTargets(targets, action)
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

// setTimeFilterWithInfo keeps the time filter of the scan for display, and
// disables deleting while it is active, unless that is allowed.
func (ui *UI) setTimeFilterWithInfo(tf *timefilter.TimeFilter, loc *time.Location) {
	ui.timeFilter = tf
	ui.timeFilterLoc = loc

	if tf != nil && !tf.IsEmpty() && !ui.isDeleteAllowedWithFilter() {
		ui.setNoDeleteWithFilter()
	}
}

// hasActiveTimeFilter returns true if any time filter is active
func (ui *UI) hasActiveTimeFilter() bool {
	return ui.timeFilter != nil && !ui.timeFilter.IsEmpty()
}

// formatTimeFilterInfo formats the time filter information for display
func (ui *UI) formatTimeFilterInfo() string {
	if !ui.hasActiveTimeFilter() {
		return ""
	}

	return ui.timeFilter.FormatForDisplay(ui.timeFilterLoc)
}

// isDeleteAllowedWithFilter checks if deletion is allowed when filters are active
func (ui *UI) isDeleteAllowedWithFilter() bool {
	if !ui.hasActiveTimeFilter() {
		return true
	}

	// Check environment variable override
	if os.Getenv("GDU_ALLOW_DELETE_WITH_FILTER") == "1" {
		return true
	}

	return false
}

// sanitizePathForDisplay replaces control characters (which can include
// terminal escape sequences) with the Unicode replacement character. A
// scanned directory entry's name has no character restrictions, and these
// paths are printed to the real terminal after the tview screen has
// stopped, bypassing tview's own protective rendering entirely.
func sanitizePathForDisplay(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}

// printMarkedPaths prints the paths of the marked items to the output
func (ui *UI) printMarkedPaths() {
	for _, path := range ui.markedPaths {
		fmt.Fprintf(ui.output, "%s\n", sanitizePathForDisplay(path))
	}
}
