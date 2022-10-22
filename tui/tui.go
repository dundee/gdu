package tui

import (
	"io"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
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
               [::b]B    [white:black:-]Toggle bar alignment to biggest file or directory
               [::b]c    [white:black:-]Show/hide file count
               [::b]m    [white:black:-]Show/hide latest mtime
               [::b]b    [white:black:-]Spawn shell in current directory
               [::b]q    [white:black:-]Quit gdu
               [::b]Q    [white:black:-]Quit gdu and print current directory path

Item under cursor:
               [::b]d    [white:black:-]Delete file or directory
               [::b]e    [white:black:-]Empty file or directory
               [::b]v    [white:black:-]Show content of file
               [::b]o    [white:black:-]Open file or directory in external program
               [::b]i    [white:black:-]Show info about item

Sort by (twice toggles asc/desc):
               [::b]n    [white:black:-]Sort by name (asc/desc)
               [::b]s    [white:black:-]Sort by size (asc/desc)
               [::b]C    [white:black:-]Sort by file count (asc/desc)
               [::b]M    [white:black:-]Sort by mtime (asc/desc)`

// UI struct
type UI struct {
	*common.UI
	app                     common.TermApplication
	screen                  tcell.Screen
	output                  io.Writer
	header                  *tview.TextView
	footer                  *tview.Flex
	footerLabel             *tview.TextView
	currentDirLabel         *tview.TextView
	pages                   *tview.Pages
	progress                *tview.TextView
	help                    *tview.Flex
	table                   *tview.Table
	filteringInput          *tview.InputField
	currentDir              fs.Item
	devices                 []*device.Device
	topDir                  fs.Item
	topDirPath              string
	currentDirPath          string
	askBeforeDelete         bool
	showItemCount           bool
	showMtime               bool
	filtering               bool
	filterValue             string
	sortBy                  string
	sortOrder               string
	done                    chan struct{}
	remover                 func(fs.Item, fs.Item) error
	emptier                 func(fs.Item, fs.Item) error
	getter                  device.DevicesInfoGetter
	exec                    func(argv0 string, argv []string, envv []string) error
	linkedItems             fs.HardLinkedItems
	selectedTextColor       tcell.Color
	selectedBackgroundColor tcell.Color
	currentItemNameMaxLen   int
	defaultSortBy           string
	defaultSortOrder        string
}

// Option is optional function customizing the bahaviour of UI
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
		remover:                 analyze.RemoveItemFromDir,
		emptier:                 analyze.EmptyFileFromDir,
		exec:                    Execute,
		linkedItems:             make(fs.HardLinkedItems, 10),
		selectedTextColor:       tview.Styles.TitleColor,
		selectedBackgroundColor: tview.Styles.MoreContrastBackgroundColor,
		currentItemNameMaxLen:   70,
		defaultSortBy:           "size",
		defaultSortOrder:        "desc",
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
			Foreground(ui.selectedTextColor).
			Background(ui.selectedBackgroundColor).Bold(true))
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

// SetSelectedTextColor sets the color for the highighted selected text
func (ui *UI) SetSelectedTextColor(color tcell.Color) {
	ui.selectedTextColor = color
}

// SetSelectedBackgroundColor sets the color for the highighted selected text
func (ui *UI) SetSelectedBackgroundColor(color tcell.Color) {
	ui.selectedBackgroundColor = color
}

// SetCurrentItemNameMaxLen sets the maximum length of the path of the currently processed item
// to be shown in the progress modal
func (ui *UI) SetCurrentItemNameMaxLen(len int) {
	ui.currentItemNameMaxLen = len
}

// StartUILoop starts tview application
func (ui *UI) StartUILoop() error {
	return ui.app.Run()
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
		return
	}

	origDir := ui.currentDir
	selectedDir := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !selectedDir.IsDir() {
		return
	}

	ui.currentDir = selectedDir.(*analyze.Dir)
	ui.hideFilterInput()
	ui.showDir()

	if selectedDir == origDir.GetParent() {
		index, _ := ui.currentDir.GetFiles().IndexOf(origDir)
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

	ui.resetSorting()

	ui.Analyzer.ResetProgress()
	ui.linkedItems = make(fs.HardLinkedItems)
	err = ui.AnalyzePath(selectedDevice.MountPoint, nil)
	if err != nil {
		ui.showErr("Error analyzing device", err)
	}
}

func (ui *UI) confirmDeletion(shouldEmpty bool) {
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
