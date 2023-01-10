package testapp

import (
	"errors"
	"sync"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CreateSimScreen returns tcell.SimulationScreen
func CreateSimScreen(width, height int) tcell.SimulationScreen {
	screen := tcell.NewSimulationScreen("UTF-8")
	err := screen.Init()
	if err != nil {
		panic(err)
	}
	screen.SetSize(width, height)

	return screen
}

// CreateTestAppWithSimScreen returns app with simulation screen for tests
func CreateTestAppWithSimScreen(width, height int) (*tview.Application, tcell.SimulationScreen) {
	app := tview.NewApplication()
	screen := CreateSimScreen(width, height)
	app.SetScreen(screen)
	return app, screen
}

// MockedApp is tview.Application with mocked methods
type MockedApp struct {
	FailRun     bool
	updateDraws []func()
	BeforeDraws []func(screen tcell.Screen) bool
	mutex       *sync.Mutex
}

// CreateMockedApp returns app with simulation screen for tests
func CreateMockedApp(failRun bool) common.TermApplication {
	app := &MockedApp{
		FailRun:     failRun,
		updateDraws: make([]func(), 0, 1),
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

// Suspend runs given function
func (app *MockedApp) Suspend(f func()) bool {
	f()
	return true
}

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

// SetMouseCapture does nothing
func (app *MockedApp) SetMouseCapture(
	capture func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction),
) *tview.Application {
	return nil
}

// QueueUpdateDraw does nothing
func (app *MockedApp) QueueUpdateDraw(f func()) *tview.Application {
	app.mutex.Lock()
	app.updateDraws = append(app.updateDraws, f)
	app.mutex.Unlock()
	return nil
}

// QueueUpdateDraw does nothing
func (app *MockedApp) GetUpdateDraws() []func() {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	return app.updateDraws
}

// SetBeforeDrawFunc does nothing
func (app *MockedApp) SetBeforeDrawFunc(f func(screen tcell.Screen) bool) *tview.Application {
	app.BeforeDraws = append(app.BeforeDraws, f)
	return nil
}
