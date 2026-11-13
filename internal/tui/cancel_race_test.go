package tui

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

type cancelRaceApp struct {
	*testapp.MockedApp
	updates chan func()
	mu      sync.Mutex
	stops   int
}

func newCancelRaceApp() *cancelRaceApp {
	return &cancelRaceApp{
		MockedApp: &testapp.MockedApp{},
		updates:   make(chan func(), 1024),
	}
}

func (app *cancelRaceApp) QueueUpdateDraw(update func()) *tview.Application {
	app.updates <- update
	return nil
}

func (app *cancelRaceApp) Stop() {
	app.mu.Lock()
	app.stops++
	app.mu.Unlock()
}

func (app *cancelRaceApp) stopCount() int {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.stops
}

func (app *cancelRaceApp) nextUpdate(t *testing.T) func() {
	t.Helper()
	select {
	case update := <-app.updates:
		return update
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queued UI update")
		return nil
	}
}

type failOnceSignalScreen struct {
	tcell.Screen
	attempted chan struct{}
	fail      bool
}

func (screen *failOnceSignalScreen) PostEvent(event tcell.Event) error {
	if screen.fail {
		screen.fail = false
		close(screen.attempted)
		return tcell.ErrEventQFull
	}
	return screen.Screen.PostEvent(event)
}

type signalLifecycleApp struct {
	*testapp.MockedApp
	signalAttempted <-chan struct{}
}

func (app *signalLifecycleApp) Run() error {
	<-app.signalAttempted
	return nil
}

// cancelRaceAnalyzer scans until it is released or its scan is cancelled,
// and counts the cancellations it sees while scanning.
type cancelRaceAnalyzer struct {
	*testanalyze.MockedAnalyzer
	started   chan struct{}
	release   chan struct{}
	cancelled chan struct{}
	cancelMu  sync.Mutex
	cancels   int
	once      sync.Once
}

func newCancelRaceAnalyzer() *cancelRaceAnalyzer {
	return &cancelRaceAnalyzer{
		MockedAnalyzer: &testanalyze.MockedAnalyzer{},
		started:        make(chan struct{}),
		release:        make(chan struct{}),
		cancelled:      make(chan struct{}),
	}
}

func (a *cancelRaceAnalyzer) AnalyzeDir(ctx context.Context, path string, opts analyze.Options) fs.Item {
	stop := context.AfterFunc(ctx, a.cancel)
	defer stop()
	close(a.started)
	select {
	case <-a.cancelled:
	case <-a.release:
	}
	return a.MockedAnalyzer.AnalyzeDir(ctx, path, opts)
}

func (a *cancelRaceAnalyzer) cancel() {
	a.cancelMu.Lock()
	a.cancels++
	a.cancelMu.Unlock()
	a.once.Do(func() { close(a.cancelled) })
}

func (a *cancelRaceAnalyzer) cancelCount() int {
	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	return a.cancels
}

func newCancelRaceUI(app *cancelRaceApp, analyzer *cancelRaceAnalyzer) *UI {
	screen := testapp.CreateSimScreen()
	ui := CreateUI(app, screen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = analyzer
	ui.done = make(chan struct{})
	return ui
}

func drainScanUpdates(t *testing.T, app *cancelRaceApp, ui *UI) {
	t.Helper()
	// Progress updates may come before the one applying the completed scan,
	// as many as the scan took ticks of the progress observer.
	for ui.scanning {
		app.nextUpdate(t)()
	}
	require.False(t, ui.pages.HasPage("progress"))
}

// requireCancelledOnce waits until the analyzer has been cancelled, which the
// scan does from a goroutine of its own, and checks it was cancelled once.
func requireCancelledOnce(t *testing.T, analyzer *cancelRaceAnalyzer) {
	t.Helper()
	require.Eventually(t, func() bool { return analyzer.cancelCount() > 0 }, 2*time.Second, time.Millisecond)
	require.Equal(t, 1, analyzer.cancelCount())
}

func TestAnalyzePathImmediateCtrlCIsNotLost(t *testing.T) {
	app := newCancelRaceApp()
	analyzer := newCancelRaceAnalyzer()
	ui := newCancelRaceUI(app, analyzer)

	require.NoError(t, ui.AnalyzePath("test_dir", nil))
	// Ctrl-C is intentionally delivered immediately, before AnalyzeDir is
	// guaranteed to have started. Cancel must still be observed by the double.
	require.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0)))
	// The second press escalates to quitting, but must not cancel twice nor
	// make the in-flight scan unsafe.
	_ = ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))

	select {
	case <-analyzer.started:
	case <-time.After(2 * time.Second):
		t.Fatal("AnalyzeDir did not start")
	}
	select {
	case <-ui.done:
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not complete after cancellation")
	}
	drainScanUpdates(t, app, ui)

	require.Equal(t, 1, analyzer.cancelCount())
	require.Equal(t, 1, app.stopCount())
	require.Equal(t, "test_dir", ui.currentDir.GetName())
}

func TestCtrlCBeforeQueuedCompletionKeepsResults(t *testing.T) {
	app := newCancelRaceApp()
	analyzer := newCancelRaceAnalyzer()
	ui := newCancelRaceUI(app, analyzer)

	require.NoError(t, ui.AnalyzePath("test_dir", nil))
	close(analyzer.release)
	select {
	case <-ui.done:
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not complete")
	}

	// The completion draw is queued, but has not run yet. Cancellation must
	// not discard the result or make the completion callback unsafe.
	require.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0)))
	drainScanUpdates(t, app, ui)

	// the scan was over, so there was nothing left to cancel
	require.Equal(t, 0, analyzer.cancelCount())
	require.Equal(t, "test_dir", ui.currentDir.GetName())
	require.Equal(t, 4, ui.table.GetRowCount())
}

func TestRepeatedSignalCtrlCCancelsOnceThenQuits(t *testing.T) {
	app := newCancelRaceApp()
	analyzer := newCancelRaceAnalyzer()
	ui := newCancelRaceUI(app, analyzer)

	require.NoError(t, ui.AnalyzePath("test_dir", nil))
	ui.keyPressed(signalEvent(os.Interrupt))
	requireCancelledOnce(t, analyzer)
	require.Equal(t, 0, app.stopCount())

	// The second SIGINT arrives while the scan is still stopping. Cancellation
	// stays idempotent, but the user's escalation is honoured by quitting.
	ui.keyPressed(signalEvent(os.Interrupt))
	require.Equal(t, 1, analyzer.cancelCount())
	require.Equal(t, 1, app.stopCount())

	close(analyzer.release)
	select {
	case <-ui.done:
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not complete")
	}
	drainScanUpdates(t, app, ui)

	// Once the completion callback has marked the scan finished, SIGINT still
	// takes the normal quit path.
	ui.keyPressed(signalEvent(os.Interrupt))
	require.Equal(t, 2, app.stopCount())
}

func TestSignalLoopQueuesScanCancellation(t *testing.T) {
	app := newCancelRaceApp()
	analyzer := &testanalyze.MockedAnalyzer{}
	screen := testapp.CreateSimScreen()
	require.NoError(t, screen.Init())
	defer screen.Fini()
	ui := CreateUI(app, screen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = analyzer
	ui.progress = tview.NewTextView()
	scan := fakeRunningScan(ui)

	signals := make(chan os.Signal, 1)
	signals <- os.Interrupt
	close(signals)
	ui.handleSignals(signals, make(chan struct{}))

	event, ok := screen.PollEvent().(*tcell.EventKey)
	require.True(t, ok)
	ui.keyPressed(event)

	require.Error(t, scan.Err(), "the scan was not cancelled")
	require.True(t, ui.scanCancelled)
	require.Equal(t, 0, app.stopCount())
}

func TestSignalLoopRetriesFullEventQueue(t *testing.T) {
	app := newCancelRaceApp()
	analyzer := &testanalyze.MockedAnalyzer{}
	simScreen := testapp.CreateSimScreen()
	require.NoError(t, simScreen.Init())
	defer simScreen.Fini()
	screen := &failOnceSignalScreen{
		Screen:    simScreen,
		attempted: make(chan struct{}),
		fail:      true,
	}
	ui := CreateUI(app, screen, &bytes.Buffer{}, true, true, false, false)
	ui.Analyzer = analyzer
	ui.progress = tview.NewTextView()
	scan := fakeRunningScan(ui)

	signals := make(chan os.Signal, 1)
	signals <- os.Interrupt
	close(signals)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ui.handleSignals(signals, make(chan struct{}))
	}()

	select {
	case <-screen.attempted:
	case <-time.After(time.Second):
		t.Fatal("signal event delivery was not attempted")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("signal event was not retried")
	}

	event, ok := simScreen.PollEvent().(*tcell.EventKey)
	require.True(t, ok)
	ui.keyPressed(event)

	require.Error(t, scan.Err(), "the scan was not cancelled")
	require.True(t, ui.scanCancelled)
	require.Equal(t, 0, app.stopCount())
}
