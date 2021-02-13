package common

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Application is interface for the UI app
type Application interface {
	Run() error
	Stop()
	SetRoot(root tview.Primitive, fullscreen bool) *tview.Application
	SetFocus(p tview.Primitive) *tview.Application
	SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) *tview.Application
	QueueUpdateDraw(f func()) *tview.Application
}
