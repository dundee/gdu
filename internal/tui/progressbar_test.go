package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/session"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
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

// scanWithDeviceSize scans test_dir as if it were a device of the given size.
func scanWithDeviceSize(t *testing.T, showBar bool) *UI {
	t.Helper()
	simScreen := testapp.CreateSimScreen()
	t.Cleanup(simScreen.Fini)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, nil, false, false, false, false)
	ui.currentDeviceSize = 1000
	ui.showDiskProgressBar = showBar
	ui.done = make(chan struct{})

	require.NoError(t, ui.AnalyzePath("test_dir", nil))
	<-ui.done
	for _, draw := range app.GetUpdateDraws() {
		draw()
	}
	return ui
}

func TestScanWithDiskProgressClearsDeviceSize(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := scanWithDeviceSize(t, true)

	assert.Equal(t, int64(0), ui.currentDeviceSize)
}

func TestScanWithoutDiskProgressKeepsDeviceSize(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := scanWithDeviceSize(t, false)

	assert.Equal(t, int64(1000), ui.currentDeviceSize)
}

func TestProgressObserverUpdatesBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, nil, false, false, false, false)
	ui.progressBar = NewProgressBar()
	ui.showScanProgressModal(" Scanning... ")

	observe := ui.progressObserver(1000, true)
	observe(session.Progress{Progress: analyze.Progress{ItemCount: 7, TotalUsage: 420}, Root: 1, Roots: 1})
	for _, draw := range app.GetUpdateDraws() {
		draw()
	}

	assert.Equal(t, 42, ui.progressBar.GetProgress())
	assert.Contains(t, ui.progress.GetText(false), "Total items: ")
}

func TestProgressObserverShowsTheScannedRoot(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, nil, false, false, false, false)
	ui.showScanProgressModal(scanProgressTitle(1, 2))

	observe := ui.progressObserver(0, false)
	observe(session.Progress{Root: 2, Roots: 2})
	for _, draw := range app.GetUpdateDraws() {
		draw()
	}
	assert.Equal(t, scanProgressTitle(2, 2), ui.progress.GetTitle())

	observe(session.Progress{Phase: session.Finalizing, Root: 2, Roots: 2})
	for _, draw := range app.GetUpdateDraws() {
		draw()
	}
	assert.Equal(t, " Finalizing... ", ui.progress.GetTitle())
}
