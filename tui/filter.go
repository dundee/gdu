package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) hideFilterInput() {
	ui.filterValue = ""
	ui.footer.Clear()
	ui.footer.AddItem(ui.footerLabel, 0, 1, false)
	ui.app.SetFocus(ui.table)
	ui.filteringInput = nil
	ui.filtering = false
}

func (ui *UI) showFilterInput() {
	if ui.currentDir == nil {
		return
	}

	if ui.filteringInput == nil {
		ui.filteringInput = tview.NewInputField()

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

		ui.footer.Clear()
		ui.footer.AddItem(ui.filteringInput, 0, 1, true)
		ui.footer.AddItem(ui.footerLabel, 0, 5, false)
	}
	ui.app.SetFocus(ui.filteringInput)
	ui.filtering = true
}
