package tui

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var analyzeParentPath = func(ui *UI, path string, parentDir fs.Item) error {
	return ui.AnalyzePath(path, parentDir)
}

func (ui *UI) keyPressed(key *tcell.EventKey) *tcell.EventKey {
	if ui.handleCtrlZ(key) == nil {
		return nil
	}

	if ui.pages.HasPage("file") || ui.pages.HasPage("export") {
		return key // send event to primitive
	}
	if ui.filtering || ui.typeFiltering {
		return key
	}

	key = ui.handleClosingModals(key)
	if key == nil {
		return nil
	}
	key = ui.handleInfoPageEvents(key)
	if key == nil {
		return nil
	}
	key = ui.handleQuit(key)
	if key == nil {
		return nil
	}

	if ui.pages.HasPage("confirm") {
		return ui.handleConfirmation(key)
	}

	if ui.previewing {
		return ui.handlePreviewKeys(key)
	}

	if ui.pages.HasPage("progress") ||
		ui.pages.HasPage("deleting") ||
		ui.pages.HasPage("emptying") {
		// allow peeking at the results found so far during a scan
		if key.Key() == tcell.KeyTab && ui.pages.HasPage("progress") {
			ui.enterPreview()
			return nil
		}
		return key
	}

	key = ui.handleHelp(key)
	if key == nil {
		return nil
	}

	if ui.pages.HasPage("help") {
		return key
	}

	key = ui.handleShell(key)
	if key == nil {
		return nil
	}

	key = ui.handleLeftRight(key)
	if key == nil {
		return nil
	}

	key = ui.handleFiltering(key)
	if key == nil {
		return nil
	}

	return ui.handleMainActions(key)
}

func (ui *UI) handleClosingModals(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() == tcell.KeyEsc || key.Rune() == 'q' {
		if ui.pages.HasPage("help") {
			ui.pages.RemovePage("help")
			ui.app.SetFocus(ui.table)
			return nil
		}
		if ui.pages.HasPage("info") {
			ui.pages.RemovePage("info")
			ui.app.SetFocus(ui.table)
			return nil
		}
	}
	return key
}

func (ui *UI) handleConfirmation(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == 'h' {
		return tcell.NewEventKey(tcell.KeyLeft, 0, 0)
	}
	if key.Rune() == 'l' {
		return tcell.NewEventKey(tcell.KeyRight, 0, 0)
	}
	return key
}

func (ui *UI) handleInfoPageEvents(key *tcell.EventKey) *tcell.EventKey {
	if ui.pages.HasPage("info") {
		switch key.Rune() {
		case 'i':
			ui.pages.RemovePage("info")
			ui.app.SetFocus(ui.table)
			return nil
		case '?':
			return nil
		}

		if key.Key() == tcell.KeyUp ||
			key.Key() == tcell.KeyDown ||
			key.Rune() == 'j' ||
			key.Rune() == 'k' {
			row, column := ui.table.GetSelection()
			if (key.Key() == tcell.KeyUp || key.Rune() == 'k') && row > 0 {
				row--
			} else if (key.Key() == tcell.KeyDown || key.Rune() == 'j') &&
				row+1 < ui.table.GetRowCount() {
				row++
			}
			ui.table.Select(row, column)
		}
		ui.showInfo() // refresh file info after any change
	}
	return key
}

// handle ctrl+z job control
func (ui *UI) handleCtrlZ(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() == tcell.KeyCtrlZ {
		ui.app.Suspend(func() {
			termApp := ui.app.(*tview.Application)
			termApp.Lock()
			defer termApp.Unlock()

			err := stopProcess()
			if err != nil {
				ui.showErr("Error sending STOP signal", err)
			}
		})
		return nil
	}

	return key
}

// confirmQuitMinScanDuration is the scan time above which quitting asks for
// confirmation, so a long scan is not lost by an accidental key press.
const confirmQuitMinScanDuration = 3 * time.Second

func (ui *UI) handleQuit(key *tcell.EventKey) *tcell.EventKey {
	clearTerminalProgress()

	// do not re-trigger quitting while a confirmation dialog is open
	if ui.pages.HasPage("confirm") {
		return key
	}

	switch key.Rune() {
	case 'Q':
		ui.quit(true)
		return nil
	case 'q':
		ui.quit(false)
		return nil
	}
	return key
}

// quit asks for confirmation when there are scan results worth protecting,
// otherwise it quits immediately.
func (ui *UI) quit(printCurrentDirPath bool) {
	if ui.shouldConfirmQuit() {
		ui.confirmQuitDialog(printCurrentDirPath)
		return
	}
	ui.doQuit(printCurrentDirPath)
}

// shouldConfirmQuit returns true when quitting would discard the work of a scan
// that took a noticeable amount of time, whether it is still running or already
// finished.
func (ui *UI) shouldConfirmQuit() bool {
	if !ui.confirmQuit {
		return false
	}
	if ui.scanning {
		return time.Since(ui.scanStart) >= confirmQuitMinScanDuration
	}
	return ui.currentDir != nil && ui.scanDuration >= confirmQuitMinScanDuration
}

func (ui *UI) doQuit(printCurrentDirPath bool) {
	ui.app.Stop()
	ui.printMarkedPaths()
	if printCurrentDirPath {
		fmt.Fprintf(ui.output, "%s\n", ui.currentDirPath)
	}
}

func (ui *UI) confirmQuitDialog(printCurrentDirPath bool) {
	var text string
	if ui.scanning {
		text = "A scan has been running for " +
			time.Since(ui.scanStart).Round(time.Second).String() + ".\n\n" +
			"Do you really want to quit gdu and abandon it?"
	} else {
		text = "Do you really want to quit gdu?\n\n" +
			"This scan took " + ui.scanDuration.Round(time.Second).String() +
			" and the results are not saved.\n" +
			"Choose \"no\" and press E to export them first."
	}
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"no", "yes", "don't ask me again"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("confirm")
			ui.app.SetFocus(ui.table)
			switch buttonIndex {
			case 2:
				ui.confirmQuit = false
				fallthrough
			case 1:
				ui.doQuit(printCurrentDirPath)
			}
		})

	if !ui.UseColors {
		modal.SetBackgroundColor(tcell.ColorGray)
	} else {
		modal.SetBackgroundColor(tcell.ColorBlack)
	}
	modal.SetBorderColor(tcell.ColorDefault)

	ui.pages.AddPage("confirm", modal, true, true)
}

// enterPreview switches from the scanning progress modal to a read-only,
// point-in-time view of the directory tree discovered so far. The view does not
// auto-refresh: pressing Tab again returns to the progress modal, and entering
// the preview once more takes a fresh snapshot.
func (ui *UI) enterPreview() {
	analyzer, ok := ui.Analyzer.(interface{ GetCurrentDir() fs.Item })
	if !ok {
		return
	}
	root := analyzer.GetCurrentDir()
	if root == nil {
		return // nothing scanned yet
	}

	// compute aggregated sizes on the partial tree using a throwaway hard-link
	// map so the running scan's accounting is left untouched
	root.UpdateStats(make(fs.HardLinkedItems))

	ui.previewing = true
	ui.previewSavedDir = ui.currentDir
	ui.currentDir = root
	ui.markedRows = make(map[int]struct{})
	ui.ignoredRows = make(map[int]struct{})
	ui.pages.RemovePage("progress")
	ui.showDir()
	ui.table.Select(0, 0)
	ui.app.SetFocus(ui.table)
}

// exitPreview leaves the mid-scan preview and restores the scanning progress modal.
func (ui *UI) exitPreview() {
	ui.previewing = false
	ui.currentDir = ui.previewSavedDir
	ui.previewSavedDir = nil
	if ui.progressFlex != nil {
		ui.pages.AddPage("progress", ui.progressFlex, true, true)
	}
	ui.app.SetFocus(ui.table)
}

// handlePreviewKeys handles input while a mid-scan preview is shown. Navigation
// and sorting are allowed; destructive or external actions are intentionally
// ignored because the tree is still being built.
func (ui *UI) handlePreviewKeys(key *tcell.EventKey) *tcell.EventKey {
	if ui.pages.HasPage("help") {
		return key
	}

	if key.Key() == tcell.KeyTab || key.Key() == tcell.KeyEsc {
		ui.exitPreview()
		return nil
	}
	if key.Key() == tcell.KeyLeft {
		ui.previewLeft()
		return nil
	}
	if key.Key() == tcell.KeyRight {
		ui.handleRight()
		return nil
	}

	switch key.Rune() {
	case '?':
		ui.showHelp()
		return nil
	case 'h':
		ui.previewLeft()
		return nil
	case 'l':
		ui.handleRight()
		return nil
	case 's', 'C', 'n', 'M':
		ui.handleSorting(key)
		return nil
	case 'a', 'B', 'c', 'm':
		ui.handleToggles(key)
		return nil
	}

	// up/down/pgup/pgdn and Enter are handled by the table itself
	return key
}

// previewLeft navigates to the parent dir within the preview, or leaves the
// preview when already at its root.
func (ui *UI) previewLeft() {
	if ui.currentDir == nil || ui.currentDir.GetParent() == nil {
		ui.exitPreview()
		return
	}
	ui.fileItemSelected(0, 0)
}

func (ui *UI) handleHelp(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == '?' {
		if ui.pages.HasPage("help") {
			ui.pages.RemovePage("help")
			ui.app.SetFocus(ui.table)
			return nil
		}
		ui.showHelp()
		return nil
	}
	return key
}

func (ui *UI) handleShell(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == 'b' {
		if ui.isInArchive() {
			ui.showErr("Spawning shell is not supported in archives", nil)
			return nil
		}
		if ui.noSpawnShell {
			previousHeaderText := ui.header.GetText(false)

			// show feedback to user
			ui.header.SetText(" Shell spawning is disabled!")

			go func() {
				time.Sleep(2 * time.Second)
				ui.app.QueueUpdateDraw(func() {
					ui.header.Clear()
					ui.header.SetText(previousHeaderText)
				})
			}()

			return nil
		}
		ui.spawnShell()
		return nil
	}
	return key
}

func (ui *UI) handleLeftRight(key *tcell.EventKey) *tcell.EventKey {
	if key.Rune() == 'h' || key.Key() == tcell.KeyLeft {
		ui.handleLeft()
		return nil
	}

	if key.Rune() == 'l' || key.Key() == tcell.KeyRight {
		ui.handleRight()
		return nil
	}
	return key
}

func (ui *UI) handleFiltering(key *tcell.EventKey) *tcell.EventKey {
	if key.Key() != tcell.KeyTab {
		return key
	}
	if ui.filteringInput != nil {
		ui.filtering = true
		ui.app.SetFocus(ui.filteringInput)
		return nil
	}
	if ui.typeFilteringInput != nil {
		ui.typeFiltering = true
		ui.app.SetFocus(ui.typeFilteringInput)
		return nil
	}
	return key
}

// nolint: funlen // Why: there's a lot of options to handle
func (ui *UI) handleMainActions(key *tcell.EventKey) *tcell.EventKey {
	switch key.Rune() {
	case 'd':
		if ui.isInArchive() {
			ui.showErr("Deletion is not supported in archives", nil)
			return nil
		}
		ui.handleDelete(ActionDelete)
	case 'e':
		if ui.isInArchive() {
			ui.showErr("Deletion is not supported in archives", nil)
			return nil
		}
		ui.handleDelete(ActionEmpty)
	case 'v':
		if ui.isInArchive() {
			ui.showErr("Viewing content is not supported in archives", nil)
			return nil
		}
		if ui.noViewFile {
			previousHeaderText := ui.header.GetText(false)

			ui.header.SetText(" Viewing files is disabled!")

			go func() {
				time.Sleep(2 * time.Second)
				ui.app.QueueUpdateDraw(func() {
					ui.header.Clear()
					ui.header.SetText(previousHeaderText)
				})
			}()

			return nil
		}
		ui.showFile()
	case 'o':
		if ui.noSpawnShell {
			previousHeaderText := ui.header.GetText(false)

			// show feedback to user
			ui.header.SetText(" Opening items is disabled!")

			go func() {
				time.Sleep(2 * time.Second)
				ui.app.QueueUpdateDraw(func() {
					ui.header.Clear()
					ui.header.SetText(previousHeaderText)
				})
			}()
			return nil
		}
		ui.openItem()
	case 'i':
		ui.showInfo()
	case 'a', 'B', 'c', 'm':
		ui.handleToggles(key)
	case 'r':
		if ui.currentDir != nil {
			ui.rescanDir()
		}
	case 'E':
		ui.confirmExport()
		return nil
	case 's', 'C', 'n', 'M':
		ui.handleSorting(key)
	case '/':
		ui.showFilterInput()
		return nil
	case 'T':
		ui.showTypeFilterInput()
		return nil
	case ' ':
		ui.handleMark()
	case 'p':
		ui.printMarked()
		return nil
	case 'I':
		ui.ignoreItem()
	}
	return key
}

func (ui *UI) handleToggles(key *tcell.EventKey) {
	switch key.Rune() {
	case 'a':
		ui.ShowApparentSize = !ui.ShowApparentSize
	case 'B':
		ui.ShowRelativeSize = !ui.ShowRelativeSize
	case 'c':
		ui.showItemCount = !ui.showItemCount
	case 'm':
		ui.showMtime = !ui.showMtime
	}
	if ui.currentDir != nil {
		row, column := ui.table.GetSelection()
		ui.showDir()
		ui.table.Select(row, column)
	}
}

func (ui *UI) handleSorting(key *tcell.EventKey) {
	switch key.Rune() {
	case 's':
		ui.setSorting("size")
	case 'C':
		ui.setSorting("itemCount")
	case 'n':
		ui.setSorting("name")
	case 'M':
		ui.setSorting("mtime")
	}
}

func (ui *UI) handleLeft() {
	if ui.currentDirPath == ui.topDirPath {
		if ui.devices != nil {
			ui.currentDir = nil
			err := ui.ListDevices(ui.getter)
			if err != nil {
				ui.showErr("Error listing devices", err)
			}
		} else if ui.browseParentDirs {
			ui.analyzeParentOfTopDir()
		}
		return
	}
	if ui.currentDir != nil {
		ui.fileItemSelected(0, 0)
	}
}

func (ui *UI) analyzeParentOfTopDir() {
	if ui.currentDir == nil || ui.isInArchive() {
		return
	}

	currentPath := ui.currentDir.GetPath()
	parentPath := filepath.Dir(currentPath)
	if parentPath == currentPath {
		return
	}

	ui.Analyzer.ResetProgress()
	ui.linkedItems = make(fs.HardLinkedItems)

	if err := analyzeParentPath(ui, parentPath, nil); err != nil {
		ui.showErr("Error analyzing parent directory", err)
	}
}

func (ui *UI) handleRight() {
	row, column := ui.table.GetSelection()
	if ui.currentDirPath != ui.topDirPath && row == 0 { // do not select /..
		return
	}

	if ui.currentDir != nil {
		ui.fileItemSelected(row, column)
	} else {
		ui.deviceItemSelected(row, column)
	}
}

func (ui *UI) handleDelete(action DeleteAction) {
	if ui.currentDir == nil {
		return
	}
	// do not allow deleting parent dir
	row, column := ui.table.GetSelection()
	selectedFile, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedFile == ui.currentDir.GetParent() {
		return
	}

	if ui.askBeforeDelete {
		ui.confirmDeletion(action)
	} else {
		ui.delete(action)
	}
}

func (ui *UI) handleMark() {
	if ui.currentDir == nil {
		return
	}
	// do not allow deleting parent dir
	row, column := ui.table.GetSelection()
	selectedFile, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedFile == ui.currentDir.GetParent() {
		return
	}

	ui.fileItemMarked(row)
}

func (ui *UI) ignoreItem() {
	if ui.currentDir == nil {
		return
	}
	// do not allow ignoring parent dir
	row, column := ui.table.GetSelection()
	selectedFile, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok || selectedFile == ui.currentDir.GetParent() {
		return
	}

	if _, ok := ui.ignoredRows[row]; ok {
		delete(ui.ignoredRows, row)
	} else {
		ui.ignoredRows[row] = struct{}{}
	}
	ui.showDir()
	// select next row if possible
	ui.table.Select(min(row+1, ui.table.GetRowCount()-1), 0)
}
