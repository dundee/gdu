package tui

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/gdamore/tcell/v2"
)

// screenLines returns the text currently shown on the simulated screen.
func screenLines(screen tcell.SimulationScreen) []string {
	cells, width, height := screen.GetContents()
	lines := make([]string, height)
	for y := 0; y < height; y++ {
		var line strings.Builder
		for x := 0; x < width; x++ {
			line.Write(cells[y*width+x].Bytes)
		}
		lines[y] = line.String()
	}
	return lines
}

// renderedForeground returns the foreground colour of the first cell of the
// given text on screen. It searches runes, not bytes, because the size bars
// contain multi-byte characters.
func renderedForeground(screen tcell.SimulationScreen, substr string) (tcell.Color, bool) {
	cells, width, _ := screen.GetContents()
	needle := []rune(substr)
	for y, line := range screenLines(screen) {
		runes := []rune(line)
		for x := 0; x+len(needle) <= len(runes); x++ {
			if string(runes[x:x+len(needle)]) != substr {
				continue
			}
			foreground, _, _ := cells[y*width+x].Style.Decompose()
			return foreground, true
		}
	}
	return tcell.ColorDefault, false
}

// TestHiddenDeletionShowsPopupAndDimsQueuedRows drives the real tview event
// loop with key presses, because the two regressions it guards against only
// showed up in a rendered frame: the bottom-right popup was never drawn, and a
// queued item kept the selected-row colour instead of the queued colour.
func TestHiddenDeletionShowsPopupAndDimsQueuedRows(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	require.NoError(t, os.MkdirAll("test_dir/other", 0o755))

	app, screen := testapp.CreateTestAppWithSimScreen(80, 30)
	defer screen.Fini()

	ui := CreateUI(app, screen, &bytes.Buffer{}, true, true, false, false)
	ui.done = make(chan struct{})

	stopped := make(chan struct{})
	go func() {
		_ = app.Run()
		close(stopped)
	}()
	defer func() {
		app.Stop()
		<-stopped
	}()

	syncApp := func() {
		done := make(chan struct{})
		app.QueueUpdateDraw(func() { close(done) })
		<-done
	}
	syncApp()

	require.NoError(t, ui.AnalyzePath("test_dir", nil))
	<-ui.done
	syncApp()

	started := make(chan struct{})
	proceed := make(chan struct{})
	var once sync.Once
	ui.newDeleter = fakeDeleter(func(dir, item fs.Item) error {
		once.Do(func() { close(started) })
		<-proceed
		return nil
	})
	release := sync.OnceFunc(func() { close(proceed) })
	defer release()

	// Start a foreground deletion through the normal confirmation dialog.
	app.QueueUpdateDraw(func() {
		ui.table.Select(rowOf(t, ui, "nested"), 0)
	})
	app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'd', 0))
	syncApp()
	syncApp()
	app.QueueEvent(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the deletion did not start")
	}
	syncApp()

	// Hiding the dialog must show the popup in the bottom-right corner.
	app.QueueEvent(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	syncApp()
	syncApp()
	require.Contains(t, strings.Join(screenLines(screen), "\n"), "Deleting:")

	// The item currently being deleted is dimmed too.
	deleting, ok := renderedForeground(screen, "nested")
	require.True(t, ok, "the deleting row is not on screen")
	require.Equal(t, ui.queuedTextColor, deleting)

	// Queue another deletion and leave the cursor on it: the queued colour
	// must still win over the selected-row style.
	app.QueueUpdateDraw(func() {
		ui.table.Select(rowOf(t, ui, "other"), 0)
	})
	syncApp()
	app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'd', 0))
	syncApp()
	syncApp()
	app.QueueEvent(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	job := ui.activeDeletionJob()
	require.Eventually(t, func() bool {
		job.mut.Lock()
		defer job.mut.Unlock()
		return len(job.pending) == 1
	}, 3*time.Second, 10*time.Millisecond)
	syncApp()
	syncApp()

	queued, ok := renderedForeground(screen, "other")
	require.True(t, ok, "the queued row is not on screen")
	require.Equal(t, ui.queuedTextColor, queued)

	deleting, ok = renderedForeground(screen, "nested")
	require.True(t, ok, "the deleting row is not on screen")
	require.Equal(t, ui.queuedTextColor, deleting)

	// Pressing the delete key on the queued item cancels it and restores the
	// normal colour.
	app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'd', 0))
	require.Eventually(t, func() bool {
		job.mut.Lock()
		defer job.mut.Unlock()
		return len(job.pending) == 0
	}, 3*time.Second, 10*time.Millisecond)
	syncApp()
	syncApp()

	restored, ok := renderedForeground(screen, "other")
	require.True(t, ok, "the restored row is not on screen")
	require.NotEqual(t, ui.queuedTextColor, restored)

	release()
	<-ui.done
	syncApp()
}
