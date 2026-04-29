package tui

import (
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/stretchr/testify/assert"
)

func TestProgressBarSetGetProgress(t *testing.T) {
	pb := NewProgressBar()
	assert.Equal(t, 0, pb.GetProgress())

	pb.SetProgress(50)
	assert.Equal(t, 50, pb.GetProgress())

	pb.SetProgress(100)
	assert.Equal(t, 100, pb.GetProgress())
}

func TestProgressBarClampsProgress(t *testing.T) {
	pb := NewProgressBar()

	pb.SetProgress(-10)
	assert.Equal(t, 0, pb.GetProgress())

	pb.SetProgress(150)
	assert.Equal(t, 100, pb.GetProgress())
}

func TestProgressBarDraw(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	pb := NewProgressBar()
	pb.SetBorder(true)
	pb.SetProgress(42)
	pb.SetRect(0, 0, 40, 3)
	pb.Draw(simScreen)
	simScreen.Show()

	// If Draw completed without panic, the test passes.
	assert.Equal(t, 42, pb.GetProgress())
}

func TestProgressBarDrawWithColor(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	pb := NewProgressBar()
	pb.SetBorder(true)
	pb.SetUseColor(true)
	pb.SetProgress(75)
	pb.SetRect(0, 0, 40, 3)
	pb.Draw(simScreen)
	simScreen.Show()

	assert.Equal(t, 75, pb.GetProgress())
}

func TestUpdateProgressWithDeviceSize(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, nil, false, false, false, false)
	ui.currentDeviceSize = 1000
	ui.showDiskProgressBar = true

	done := ui.Analyzer.GetDone()
	done.Broadcast()
	ui.updateProgress(ui.Analyzer, done)

	// After updateProgress returns, currentDeviceSize must be cleared.
	assert.Equal(t, int64(0), ui.currentDeviceSize)
}

func TestUpdateProgressBarDisabled(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, nil, false, false, false, false)
	ui.currentDeviceSize = 1000
	ui.showDiskProgressBar = false

	done := ui.Analyzer.GetDone()
	done.Broadcast()
	ui.updateProgress(ui.Analyzer, done)

	// showDiskProgressBar is false, so currentDeviceSize must NOT be cleared.
	assert.Equal(t, int64(1000), ui.currentDeviceSize)
}

func TestUpdateProgressUpdatesBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, nil, false, false, false, false)
	ui.currentDeviceSize = 1000
	ui.showDiskProgressBar = true
	ui.progressBar = NewProgressBar()

	doneChan := ui.Analyzer.GetDone()

	go func() {
		// Wait for updateProgress to start polling
		time.Sleep(150 * time.Millisecond)
		doneChan.Broadcast()
	}()

	ui.updateProgress(ui.Analyzer, doneChan)
}
