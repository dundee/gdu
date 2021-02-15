package testapp

import (
	"errors"

	"github.com/dundee/gdu/common"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CreateTestAppWithSimScreen returns app with simulation screen for tests
func CreateTestAppWithSimScreen(width, height int) (*tview.Application, tcell.SimulationScreen) {
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	screen.SetSize(width, height)

	app := tview.NewApplication()
	app.SetScreen(screen)

	return app, screen
}

// CreateMockedApp returns app with simulation screen for tests
func CreateMockedApp(failRun bool) common.Application {
	app := &MockedApp{
		FailRun: failRun,
	}
	return app
}

// MockedApp is tview.Application with mocked methods
type MockedApp struct {
	FailRun bool
}

// Run does nothing
func (app *MockedApp) Run() error {
	if app.FailRun {
		return errors.New("Fail")
	}

	return nil
}

// Stop does nothing
func (app *MockedApp) Stop() {}

// SetRoot does nothing
func (app *MockedApp) SetRoot(root tview.Primitive, fullscreen bool) *tview.Application {
	return nil
}

// SetFocus does nothing
func (app *MockedApp) SetFocus(p tview.Primitive) *tview.Application {
	return nil
}

// SetInputCapture does nothing
func (app *MockedApp) SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) *tview.Application {
	return nil
}

// QueueUpdateDraw does nothing
func (app *MockedApp) QueueUpdateDraw(f func()) *tview.Application {
	return nil
}

// SetBeforeDrawFunc does nothing
func (app *MockedApp) SetBeforeDrawFunc(func(screen tcell.Screen) bool) *tview.Application {
	return nil
}
