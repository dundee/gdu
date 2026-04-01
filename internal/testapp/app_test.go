package testapp

import (
	"sync"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

// Compile-time check that MockedApp implements common.TermApplication
var _ common.TermApplication = (*MockedApp)(nil)

func TestCreateSimScreen(t *testing.T) {
	screen := CreateSimScreen()

	assert.NotNil(t, screen)
}

func TestCreateTestAppWithSimScreen(t *testing.T) {
	app, screen := CreateTestAppWithSimScreen(120, 40)

	assert.NotNil(t, app)
	assert.NotNil(t, screen)

	w, h := screen.Size()
	assert.Equal(t, 120, w)
	assert.Equal(t, 40, h)
}

func TestCreateMockedApp(t *testing.T) {
	app := CreateMockedApp(false)

	assert.NotNil(t, app)
}

func TestMockedAppRunSuccess(t *testing.T) {
	app := CreateMockedApp(false)

	err := app.Run()

	assert.NoError(t, err)
}

func TestMockedAppRunFail(t *testing.T) {
	app := CreateMockedApp(true)

	err := app.Run()

	assert.Error(t, err)
	assert.Equal(t, "Fail", err.Error())
}

func TestMockedAppStop(t *testing.T) {
	app := CreateMockedApp(false)

	assert.NotPanics(t, func() {
		app.Stop()
	})
}

func TestMockedAppSuspend(t *testing.T) {
	app := CreateMockedApp(false)

	called := false
	result := app.Suspend(func() {
		called = true
	})

	assert.True(t, called)
	assert.True(t, result)
}

func TestMockedAppSetRoot(t *testing.T) {
	app := CreateMockedApp(false)

	result := app.SetRoot(nil, true)

	assert.Nil(t, result)
}

func TestMockedAppSetFocus(t *testing.T) {
	app := CreateMockedApp(false)

	result := app.SetFocus(nil)

	assert.Nil(t, result)
}

func TestMockedAppSetInputCapture(t *testing.T) {
	app := CreateMockedApp(false)

	result := app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return event
	})

	assert.Nil(t, result)
}

func TestMockedAppSetMouseCapture(t *testing.T) {
	app := CreateMockedApp(false)

	result := app.SetMouseCapture(
		func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
			return event, action
		},
	)

	assert.Nil(t, result)
}

func TestMockedAppQueueUpdateDraw(t *testing.T) {
	app := CreateMockedApp(false).(*MockedApp)

	counter := 0
	f1 := func() { counter++ }
	f2 := func() { counter += 10 }

	app.QueueUpdateDraw(f1)
	app.QueueUpdateDraw(f2)

	draws := app.GetUpdateDraws()
	assert.Len(t, draws, 2)

	// Execute the queued functions and verify they work
	for _, f := range draws {
		f()
	}
	assert.Equal(t, 11, counter)
}

func TestMockedAppSetBeforeDrawFunc(t *testing.T) {
	app := CreateMockedApp(false).(*MockedApp)

	assert.Empty(t, app.BeforeDraws)

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool { return true })
	assert.Len(t, app.BeforeDraws, 1)

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool { return false })
	assert.Len(t, app.BeforeDraws, 2)
}

func TestMockedAppConcurrentQueueUpdateDraw(t *testing.T) {
	app := CreateMockedApp(false).(*MockedApp)

	var wg sync.WaitGroup
	n := 100
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			app.QueueUpdateDraw(func() {})
		}()
	}

	wg.Wait()

	draws := app.GetUpdateDraws()
	assert.Len(t, draws, n)
}
