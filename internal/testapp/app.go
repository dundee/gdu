package testapp

import (
	"errors"
	"sync"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CreateTestAppWithSimScreen returns app with simulation screen for tests
func CreateTestAppWithSimScreen(width, height int) (*tview.Application, tcell.SimulationScreen) {
	screen := tcell.NewSimulationScreen("UTF-8")
	err := screen.Init()
	if err != nil {
		panic(err)
	}
	screen.SetSize(width, height)

	app := tview.NewApplication()
	app.SetScreen(screen)

	return app, screen
}

// MockedApp is tview.Application with mocked methods
type MockedApp struct {
	FailRun     bool
	UpdateDraws []func()
	BeforeDraws []func(screen tcell.Screen) bool
	mutex       *sync.Mutex
}

// CreateMockedApp returns app with simulation screen for tests
func CreateMockedApp(failRun bool) common.TermApplication {
	app := &MockedApp{
		FailRun:     failRun,
		UpdateDraws: make([]func(), 0, 1),
		BeforeDraws: make([]func(screen tcell.Screen) bool, 0, 1),
		mutex:       &sync.Mutex{},
	}
	return app
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
	app.mutex.Lock()
	app.UpdateDraws = append(app.UpdateDraws, f)
	app.mutex.Unlock()
	return nil
}

// SetBeforeDrawFunc does nothing
func (app *MockedApp) SetBeforeDrawFunc(f func(screen tcell.Screen) bool) *tview.Application {
	app.BeforeDraws = append(app.BeforeDraws, f)
	return nil
}
