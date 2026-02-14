package tui

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) rebuildFooter() {
	ui.footer.Clear()
	if ui.filteringInput != nil {
		ui.footer.AddItem(ui.filteringInput, 0, 1, ui.filtering)
	}
	if ui.typeFilteringInput != nil {
		ui.footer.AddItem(ui.typeFilteringInput, 0, 1, ui.typeFiltering)
	}
	ui.footer.AddItem(ui.footerLabel, 0, 5, false)
}

func (ui *UI) hideFilterInput() {
	ui.filterValue = ""
	ui.filteringInput = nil
	ui.filtering = false
	ui.rebuildFooter()
	ui.app.SetFocus(ui.table)
}

func (ui *UI) showFilterInput() {
	if ui.currentDir == nil {
		return
	}

	if ui.filteringInput == nil {
		ui.markedRows = make(map[int]struct{})

		ui.filteringInput = tview.NewInputField()
		ui.filteringInput.SetLabel("Name: ")

		if !ui.UseColors {
			ui.filteringInput.SetFieldBackgroundColor(
				tcell.NewRGBColor(100, 100, 100),
			)
			ui.filteringInput.SetFieldTextColor(
				tcell.NewRGBColor(255, 255, 255),
			)
		}

		ui.filteringInput.SetChangedFunc(func(text string) {
			ui.filterValue = text
			ui.showDir()
		})
		ui.filteringInput.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyESC {
				ui.hideFilterInput()
				ui.showDir()
			} else {
				ui.app.SetFocus(ui.table)
				ui.filtering = false
			}
		})

		ui.rebuildFooter()
	}
	ui.app.SetFocus(ui.filteringInput)
	ui.filtering = true
}

func (ui *UI) hideTypeFilterInput() {
	ui.typeFilterValue = ""
	ui.typeFilteringInput = nil
	ui.typeFiltering = false
	ui.rebuildFooter()
	ui.app.SetFocus(ui.table)
}

func (ui *UI) showTypeFilterInput() {
	if ui.currentDir == nil {
		return
	}

	if ui.typeFilteringInput == nil {
		ui.markedRows = make(map[int]struct{})

		ui.typeFilteringInput = tview.NewInputField()
		ui.typeFilteringInput.SetLabel("Type: ")

		if !ui.UseColors {
			ui.typeFilteringInput.SetFieldBackgroundColor(
				tcell.NewRGBColor(100, 100, 100),
			)
			ui.typeFilteringInput.SetFieldTextColor(
				tcell.NewRGBColor(255, 255, 255),
			)
		}

		ui.typeFilteringInput.SetChangedFunc(func(text string) {
			ui.typeFilterValue = text
			ui.showDir()
		})
		ui.typeFilteringInput.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyESC {
				ui.hideTypeFilterInput()
				ui.showDir()
			} else {
				ui.app.SetFocus(ui.table)
				ui.typeFiltering = false
			}
		})

		ui.rebuildFooter()
	}
	ui.app.SetFocus(ui.typeFilteringInput)
	ui.typeFiltering = true
}

// matchesTypeFilter returns true if the file name matches the type filter.
// Directories always match. Files are matched by extension against the
// comma-separated list in typeFilterValue.
func (ui *UI) matchesTypeFilter(name string, isDir bool) bool {
	if ui.typeFilterValue == "" {
		return true
	}
	if isDir {
		return true
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false
	}
	ext = strings.TrimPrefix(ext, ".")

	for _, t := range strings.Split(ui.typeFilterValue, ",") {
		t = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(t), "."))
		if t != "" && t == ext {
			return true
		}
	}
	return false
}
