package common

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TermApplication is interface for the terminal UI app
type TermApplication interface {
	Run() error
	Stop()
	Suspend(f func()) bool
	SetRoot(root tview.Primitive, fullscreen bool) *tview.Application
	SetFocus(p tview.Primitive) *tview.Application
	SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) *tview.Application
	SetMouseCapture(
		capture func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction),
	) *tview.Application
	QueueUpdateDraw(f func()) *tview.Application
	SetBeforeDrawFunc(func(screen tcell.Screen) bool) *tview.Application
}
