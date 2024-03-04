package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) toggleStatusBar(show bool) {
	var textColor, textBgColor tcell.Color
	if ui.UseColors {
		textColor = tcell.NewRGBColor(0, 0, 0)
		textBgColor = tcell.NewRGBColor(36, 121, 208)
	} else {
		textColor = tcell.NewRGBColor(0, 0, 0)
		textBgColor = tcell.NewRGBColor(255, 255, 255)
	}

	ui.grid.Clear()

	if show {
		ui.status = tview.NewTextView().SetDynamicColors(true)
		ui.status.SetTextColor(textColor)
		ui.status.SetBackgroundColor(textBgColor)

		ui.grid.SetRows(1, 1, 0, 1, 1)
		ui.grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
			AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
			AddItem(ui.table, 2, 0, 1, 1, 0, 0, true).
			AddItem(ui.status, 3, 0, 1, 1, 0, 0, false).
			AddItem(ui.footer, 4, 0, 1, 1, 0, 0, false)
		return
	}
	ui.grid.SetRows(1, 1, 0, 1)
	ui.grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
		AddItem(ui.table, 2, 0, 1, 1, 0, 0, true).
		AddItem(ui.footer, 3, 0, 1, 1, 0, 0, false)
}
