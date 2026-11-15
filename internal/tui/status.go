package tui

import (
	"fmt"
	"time"

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

	ui.statusMut.Lock()
	defer ui.statusMut.Unlock()

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
	ui.status = nil
	ui.grid.SetRows(1, 1, 0, 1)
	ui.grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
		AddItem(ui.table, 2, 0, 1, 1, 0, 0, true).
		AddItem(ui.footer, 3, 0, 1, 1, 0, 0, false)
}

func (ui *UI) updateStatusWorker() {
	for {
		ui.updateStatus()
		time.Sleep(500 * time.Millisecond)
	}
}

func (ui *UI) updateStatus() {
	ui.workersMut.Lock()
	cnt := ui.activeWorkers
	ui.workersMut.Unlock()

	ui.statusMut.RLock()
	status := ui.status
	ui.statusMut.RUnlock()

	if cnt == 0 && status == nil {
		return
	}

	if cnt > 0 && status == nil {
		ui.app.QueueUpdateDraw(func() {
			ui.toggleStatusBar(true)
		})
	} else if cnt == 0 {
		ui.app.QueueUpdateDraw(func() {
			ui.toggleStatusBar(false)
		})
		return
	}

	ui.app.QueueUpdateDraw(func() {
		msg := fmt.Sprintf(" Active background deletions: %d", cnt)
		ui.statusMut.RLock()
		ui.status.SetText(msg)
		ui.statusMut.RUnlock()
	})
}
