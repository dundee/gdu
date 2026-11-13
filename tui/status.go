package tui

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// deletionStatusLinger keeps the popup visible long enough to notice even when
// the deletion itself finishes almost immediately.
const deletionStatusLinger = 1500 * time.Millisecond

// deletionStatusOverlay draws the deletion status line on top of the pages,
// anchored to the bottom right of the screen. It is not focusable, so drawing
// it never takes the keyboard away from the tree.
type deletionStatusOverlay struct {
	*tview.Pages
	status  *tview.TextView
	visible bool
}

func newDeletionStatusOverlay(pages *tview.Pages, useColors bool) *deletionStatusOverlay {
	status := tview.NewTextView()
	status.SetTextColor(tcell.NewRGBColor(0, 0, 0))
	if useColors {
		status.SetBackgroundColor(tcell.NewRGBColor(36, 121, 208))
	} else {
		status.SetBackgroundColor(tcell.NewRGBColor(255, 255, 255))
	}

	return &deletionStatusOverlay{Pages: pages, status: status}
}

// SetRect lays the pages out and keeps the status line in the bottom right
// corner, just above the footer.
func (o *deletionStatusOverlay) SetRect(x, y, width, height int) {
	o.Pages.SetRect(x, y, width, height)
	o.layout(width, height)
}

func (o *deletionStatusOverlay) Draw(screen tcell.Screen) {
	o.Pages.Draw(screen)
	if o.visible {
		o.status.Draw(screen)
	}
}

func (o *deletionStatusOverlay) layout(width, height int) {
	if !o.visible || width <= 0 || height < 2 {
		return
	}

	lineWidth := tview.TaggedStringWidth(o.status.GetText(true)) + 2
	if lineWidth > width {
		lineWidth = width
	}
	o.status.SetRect(width-lineWidth, height-2, lineWidth, 1)
}

func (o *deletionStatusOverlay) show(text string) {
	o.status.SetText(" " + text + " ")
	o.visible = true
}

func (o *deletionStatusOverlay) hide() {
	o.visible = false
}

func (ui *UI) updateStatusWorker() {
	for {
		if ui.hasDeletionActivity() {
			ui.app.QueueUpdateDraw(ui.updateDeletionStatus)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (ui *UI) hasDeletionActivity() bool {
	ui.workersMut.Lock()
	workers := ui.activeWorkers
	ui.workersMut.Unlock()
	if workers > 0 {
		return true
	}

	job := ui.activeDeletionJob()
	if job == nil {
		return false
	}
	job.mut.Lock()
	defer job.mut.Unlock()
	return job.progress.hidden
}

// updateDeletionStatus shows what is being deleted in the bottom right corner
// and hides the popup once nothing is left. A foreground deletion whose dialog
// is hidden is background activity too, so it gets a popup; while its dialog is
// visible the dialog itself is the progress display.
func (ui *UI) updateDeletionStatus() {
	if ui.statusOverlay == nil {
		return
	}

	job := ui.activeDeletionJob()
	if job != nil {
		job.mut.Lock()
		hidden := job.progress.hidden
		pending := len(job.pending)
		current := job.currentPath
		job.mut.Unlock()

		if hidden {
			text := "Deletion in progress"
			if current != "" {
				text = "Deleting: " + filepath.Base(current)
			}
			if pending > 0 {
				text += fmt.Sprintf(" (%d queued)", pending)
			}
			ui.statusOverlay.show(text)
			return
		}
	}

	ui.workersMut.Lock()
	workers := ui.activeWorkers
	ui.workersMut.Unlock()

	if workers == 0 {
		ui.statusOverlay.hide()
		return
	}
	ui.showDeletionStatus(workers)
}

// showDeletionStatus displays the popup for the given number of workers. The
// caller passes the count it observed, so an update queued while a short
// deletion was running still shows the popup even if the deletion has finished
// by the time the event loop processes the update.
func (ui *UI) showDeletionStatus(workers int) {
	if ui.statusOverlay == nil {
		return
	}

	text := fmt.Sprintf("Background deletions: %d", workers)
	if queued := len(ui.deleteQueue); queued > 0 {
		text += fmt.Sprintf(" (%d queued)", queued)
	}
	ui.statusOverlay.show(text)
}
