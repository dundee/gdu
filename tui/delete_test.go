package tui

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/pkg/remove"
	"github.com/gdamore/tcell/v2"
)

// markRow marks the item listed on row.
func markRow(t *testing.T, ui *UI, row int) {
	t.Helper()
	item, ok := ui.rowItem(row, 0)
	require.True(t, ok, "row %d lists no item", row)
	ui.markedItems[item] = struct{}{}
}

// deleterFunc is an itemDeleter which hands every deletion to a function and
// refuses nothing.
type deleterFunc func(dir, item fs.Item) error

func (f deleterFunc) Delete(dir, item fs.Item, _ remove.Mode) error { return f(dir, item) }
func (f deleterFunc) Check(fs.Item, fs.Item, remove.Mode) error     { return nil }

func fakeDeleter(f func(dir, item fs.Item) error) func([]string) itemDeleter {
	return func([]string) itemDeleter { return deleterFunc(f) }
}

// deleteNow deletes the marked items, or the selected one, without asking.
func deleteNow(ui *UI, action DeleteAction) {
	ui.deleteTargets(ui.selectedTargets(), action)
}

// drawsQueued returns how many draws the mocked app has queued so far. It
// keeps every draw, so tests run only the ones queued after this count.
func drawsQueued(ui *UI) int {
	return len(ui.app.(*testapp.MockedApp).GetUpdateDraws())
}

// runDrawsSince runs the draws queued after the first n.
func runDrawsSince(ui *UI, n int) {
	for _, draw := range ui.app.(*testapp.MockedApp).GetUpdateDraws()[n:] {
		draw()
	}
}

// waitDone waits for a deletion to finish, failing instead of hanging when it
// never does.
func waitDone(t *testing.T, ui *UI) {
	t.Helper()
	select {
	case <-ui.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the deletion did not finish")
	}
}

// rowOf returns the row listing the item named name.
func rowOf(t *testing.T, ui *UI, name string) int {
	t.Helper()
	for row := range ui.table.GetRowCount() {
		if item, ok := ui.rowItem(row, 0); ok && item.GetName() == name {
			return row
		}
	}
	t.Fatalf("no row lists %q", name)
	return -1
}

// enterNested opens test_dir/nested, which lists "/..", subnested and file2.
func enterNested(t *testing.T) *UI {
	t.Helper()
	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.fileItemSelected(rowOf(t, ui, "nested"), 0)
	require.Equal(t, "nested", ui.currentDir.GetName())
	require.True(t, ui.parentRowShown)
	return ui
}

// The confirmation must delete the item it named, even when the selection
// moves before "yes" is pressed, for instance because a background deletion
// redrew the listing.
func TestConfirmationDeletesTheItemItNamed(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := enterNested(t)
	ui.table.Select(rowOf(t, ui, "file2"), 0)
	ui.handleDelete(ActionDelete)
	require.True(t, ui.pages.HasPage("confirm"))

	// the selection moves to the other entry before "yes" is pressed
	ui.table.Select(rowOf(t, ui, "subnested"), 0)
	n := drawsQueued(ui)
	selectConfirmationYes(t, ui)
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.NoFileExists(t, "test_dir/nested/file2")
	assert.DirExists(t, "test_dir/nested/subnested")

	// and then to the "/.." row
	ui.table.Select(rowOf(t, ui, "subnested"), 0)
	ui.handleDelete(ActionDelete)
	ui.table.Select(0, 0)
	n = drawsQueued(ui)
	selectConfirmationYes(t, ui)
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.NoDirExists(t, "test_dir/nested/subnested")
	assert.DirExists(t, "test_dir/nested")
}

func TestParentRowIsNeverDeleted(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := enterNested(t)
	ui.table.Select(0, 0)

	ui.handleDelete(ActionDelete)
	assert.False(t, ui.pages.HasPage("confirm"))

	ui.askBeforeDelete = false
	ui.handleDelete(ActionDelete)
	ui.handleDelete(ActionEmpty)
	ui.handleMark()
	assert.Empty(t, ui.markedItems)
	assert.DirExists(t, "test_dir/nested")

	// even when handed in, the guard refuses the parent as an entry of the
	// directory being browsed
	n := drawsQueued(ui)
	ui.deleteTargets([]deleteTarget{{dir: ui.currentDir, item: ui.currentDir.GetParent()}}, ActionDelete)
	waitDone(t, ui)
	runDrawsSince(ui, n)
	assert.True(t, ui.pages.HasPage("error"))
	assert.DirExists(t, "test_dir/nested")
}

// Marks belong to items, not rows, so reordering the listing must not move
// them onto other items.
func TestMarksFollowTheirItemsWhenTheListingIsReordered(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := enterNested(t)
	ui.sortBy, ui.sortOrder = nameSortKey, ascOrder
	ui.showDir()
	markRow(t, ui, rowOf(t, ui, "file2"))

	ui.sortOrder = descOrder
	ui.showDir()
	n := drawsQueued(ui)
	deleteNow(ui, ActionDelete)
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.NoFileExists(t, "test_dir/nested/file2")
	assert.DirExists(t, "test_dir/nested/subnested")
}

// A collapsed row (--collapse-path) stands for the deepest directory of the
// chain, which lives below the directory being browsed.
func TestCollapsedRowDeletesTheDeepestDirectory(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	require.NoError(t, os.MkdirAll("test_dir/chain/a/b", 0o755))
	require.NoError(t, os.WriteFile("test_dir/chain/a/b/file", []byte("chain"), 0o600))

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.collapsePath = true
	ui.showDir()
	ui.table.Select(rowOf(t, ui, "b"), 0)

	n := drawsQueued(ui)
	deleteNow(ui, ActionDelete)
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.False(t, ui.pages.HasPage("error"))
	assert.NoDirExists(t, "test_dir/chain/a/b")
	assert.DirExists(t, "test_dir/chain/a")
}

func TestScannedRootIsNotDeletedFromVirtualRoot(t *testing.T) {
	first, second := createMultiTestDirs(t)
	ui := analyzeMultiplePaths(t, []string{first, second})
	ui.askBeforeDelete = false
	ui.table.Select(0, 0)

	ui.handleDelete(ActionDelete)

	assert.True(t, ui.pages.HasPage("error"))
	assert.DirExists(t, first)
	assert.DirExists(t, second)
}

func TestEmptyKeepsTheDirectory(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := enterNested(t)
	ui.table.Select(rowOf(t, ui, "subnested"), 0)
	n := drawsQueued(ui)
	deleteNow(ui, ActionEmpty)
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.DirExists(t, "test_dir/nested/subnested")
	assert.NoFileExists(t, "test_dir/nested/subnested/file")
}

func TestEmptyDoesNotWriteThroughSymlink(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	outside := filepath.Join(t.TempDir(), "precious")
	require.NoError(t, os.WriteFile(outside, []byte("precious"), 0o600))
	if err := os.Symlink(outside, "test_dir/nested/link"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	ui := enterNested(t)
	ui.table.Select(rowOf(t, ui, "link"), 0)
	n := drawsQueued(ui)
	deleteNow(ui, ActionEmpty)
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.True(t, ui.pages.HasPage("error"))
	content, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "precious", string(content))
}

func TestDeletionIsDisabledWithoutAskingToo(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.askBeforeDelete = false
	ui.noDeleteWithFilter = true
	ui.table.Select(0, 0)

	ui.handleDelete(ActionDelete)

	assert.True(t, ui.pages.HasPage("confirm"))
	assert.DirExists(t, "test_dir/nested")
}

// startPausedDeletion begins deleting the selected item and waits until the
// deleter is inside Delete, so the test can inspect the progress dialog while
// the deletion is still running.
func startPausedDeletion(t *testing.T, ui *UI, action DeleteAction) func() {
	t.Helper()
	return startPausedDeletionAt(t, ui, 0, action)
}

func startPausedDeletionAt(t *testing.T, ui *UI, row int, action DeleteAction) func() {
	t.Helper()

	started := make(chan struct{})
	proceed := make(chan struct{})
	var once sync.Once
	ui.newDeleter = fakeDeleter(func(dir, item fs.Item) error {
		once.Do(func() { close(started) })
		<-proceed
		if err := os.RemoveAll(item.GetPath()); err != nil {
			return err
		}
		dir.RemoveFile(item)
		return nil
	})

	ui.table.Select(row, 0)
	deleteNow(ui, action)
	<-started

	return sync.OnceFunc(func() { close(proceed) })
}

func TestDeletionDialogCanBeHiddenAndShownAgain(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	n := drawsQueued(ui)
	release := startPausedDeletion(t, ui, ActionDelete)

	require.NotNil(t, ui.activeDeletion)
	assert.True(t, ui.pages.HasPage("deleting"))
	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	assert.Nil(t, key)
	assert.False(t, ui.pages.HasPage("deleting"))
	assert.True(t, ui.activeDeletion.progress.hidden)

	key = ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	assert.Nil(t, key)
	assert.True(t, ui.pages.HasPage("deleting"))
	assert.False(t, ui.activeDeletion.progress.hidden)

	release()
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.Nil(t, ui.activeDeletion)
	assert.False(t, ui.pages.HasPage("deleting"))
	assert.NoDirExists(t, "test_dir/nested")
}

func TestHiddenDeletionFinishesAndClearsItsDialog(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	n := drawsQueued(ui)
	release := startPausedDeletion(t, ui, ActionDelete)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	assert.Nil(t, key)
	require.NotNil(t, ui.activeDeletion)
	assert.False(t, ui.pages.HasPage("deleting"))

	release()
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.Nil(t, ui.activeDeletion)
	assert.False(t, ui.pages.HasPage("deleting"))
	assert.NoDirExists(t, "test_dir/nested")
}

func TestAdditionalDeletionIsQueuedBehindHiddenDeletion(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	require.NoError(t, os.MkdirAll("test_dir/other", 0o755))

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	n := drawsQueued(ui)
	release := startPausedDeletionAt(t, ui, rowOf(t, ui, "nested"), ActionDelete)
	job := ui.activeDeletion
	require.NotNil(t, job)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	assert.Nil(t, key)
	assert.False(t, ui.pages.HasPage("deleting"))

	// the confirmation is unchanged, but accepting it queues the new deletion
	ui.table.Select(rowOf(t, ui, "other"), 0)
	ui.handleDelete(ActionDelete)
	require.True(t, ui.pages.HasPage("confirm"))
	selectConfirmationYes(t, ui)
	assert.Same(t, job, ui.activeDeletion)
	job.mut.Lock()
	queued := len(job.pending)
	job.mut.Unlock()
	assert.Equal(t, 1, queued)

	release()
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.Nil(t, ui.activeDeletion)
	assert.False(t, ui.pages.HasPage("deleting"))
	assert.NoDirExists(t, "test_dir/nested")
	assert.NoDirExists(t, "test_dir/other")
}

// cellForeground returns the text colour a row is drawn with.
func cellForeground(ui *UI, row int) tcell.Color {
	foreground, _, _ := ui.table.GetCell(row, 0).Style.Decompose()
	return foreground
}

func TestQueuedDeletionIsDimmedAndCanBeCancelled(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	require.NoError(t, os.MkdirAll("test_dir/other", 0o755))

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	n := drawsQueued(ui)
	release := startPausedDeletionAt(t, ui, rowOf(t, ui, "nested"), ActionDelete)
	job := ui.activeDeletion
	require.NotNil(t, job)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	assert.Nil(t, key)

	// queue another deletion while the dialog is hidden
	ui.table.Select(rowOf(t, ui, "other"), 0)
	ui.handleDelete(ActionDelete)
	require.True(t, ui.pages.HasPage("confirm"))
	selectConfirmationYes(t, ui)
	runDrawsSince(ui, n)

	otherRow := rowOf(t, ui, "other")
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, otherRow))
	// the queued colour must survive the selected-row style too, because the
	// cursor usually stays on the item the user just queued
	selectedFg, _, selectedAttrs := ui.table.GetCell(otherRow, 0).SelectedStyle.Decompose()
	if ui.UseColors {
		assert.Equal(t, ui.queuedTextColor, selectedFg)
	} else {
		assert.NotZero(t, selectedAttrs&tcell.AttrDim)
	}

	// pressing a delete key on the queued item cancels it
	ui.table.Select(otherRow, 0)
	ui.handleDelete(ActionDelete)

	assert.False(t, ui.pages.HasPage("confirm"))
	job.mut.Lock()
	pending := len(job.pending)
	job.mut.Unlock()
	assert.Equal(t, 0, pending)
	assert.Equal(t, tcell.ColorDefault, cellForeground(ui, otherRow))

	release()
	waitDone(t, ui)
	runDrawsSince(ui, n)

	assert.NoDirExists(t, "test_dir/nested")
	assert.DirExists(t, "test_dir/other")
}

// The item that is currently being deleted must be dimmed too, and redrawing
// the tree while the deletion runs must keep both it and the queued items
// dimmed.
func TestDeletionDimmingSurvivesRedraw(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	require.NoError(t, os.MkdirAll("test_dir/other", 0o755))

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	n := drawsQueued(ui)
	release := startPausedDeletionAt(t, ui, rowOf(t, ui, "nested"), ActionDelete)
	defer release()

	nestedRow := rowOf(t, ui, "nested")
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, nestedRow))

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	require.Nil(t, key)

	ui.table.Select(rowOf(t, ui, "other"), 0)
	ui.handleDelete(ActionDelete)
	require.True(t, ui.pages.HasPage("confirm"))
	selectConfirmationYes(t, ui)
	runDrawsSince(ui, n)

	// A redraw happens for example when another deletion is queued.
	ui.refreshListing()
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, nestedRow))
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, rowOf(t, ui, "other")))

	release()
	waitDone(t, ui)
	runDrawsSince(ui, n)
}

// Queueing while the dialog is hidden must refresh the listing synchronously.
// deleteTargets runs on the event loop, so a draw queued from there would wait
// for the very loop running it and freeze the UI.
func TestQueueingWhileDialogHiddenRefreshesListingImmediately(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	require.NoError(t, os.MkdirAll("test_dir/other", 0o755))

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	n := drawsQueued(ui)
	release := startPausedDeletionAt(t, ui, rowOf(t, ui, "nested"), ActionDelete)
	defer release()
	require.NotNil(t, ui.activeDeletionJob())

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	require.Nil(t, key)

	ui.table.Select(rowOf(t, ui, "other"), 0)
	ui.handleDelete(ActionDelete)
	require.True(t, ui.pages.HasPage("confirm"))
	selectConfirmationYes(t, ui)

	// No queued draw has been run, so the row is dimmed only if deleteTargets
	// refreshed the listing itself.
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, rowOf(t, ui, "other")))

	release()
	waitDone(t, ui)
	runDrawsSince(ui, n)
}

func TestDeletionStatusPopupShowsOnlyHiddenForegroundDeletion(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	require.NoError(t, os.MkdirAll("test_dir/other", 0o755))

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	n := drawsQueued(ui)
	release := startPausedDeletionAt(t, ui, rowOf(t, ui, "nested"), ActionDelete)
	require.NotNil(t, ui.statusOverlay)
	require.NotNil(t, ui.activeDeletionJob())

	// while the dialog is visible it is the progress display
	ui.updateDeletionStatus()
	assert.False(t, ui.statusOverlay.visible)

	// hiding the dialog turns the deletion into background activity
	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	assert.Nil(t, key)
	assert.True(t, ui.statusOverlay.visible)
	assert.Contains(t, ui.statusOverlay.status.GetText(true), "Deleting:")

	// showing the dialog again hides the popup
	key = ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	assert.Nil(t, key)
	assert.False(t, ui.statusOverlay.visible)

	// hide it once more and let the deletion finish while hidden
	key = ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	assert.Nil(t, key)
	assert.True(t, ui.statusOverlay.visible)

	release()
	waitDone(t, ui)
	runDrawsSince(ui, n)

	// the popup lingers briefly even though the deletion already finished
	assert.True(t, ui.statusOverlay.visible)
	ui.updateDeletionStatus()
	assert.False(t, ui.statusOverlay.visible)
	assert.NoDirExists(t, "test_dir/nested")
}

func TestDeletionStatusPopupShowsBackgroundWorkers(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	require.NotNil(t, ui.statusOverlay)

	// starting a worker updates the popup without waiting for the status poll
	n := drawsQueued(ui)
	ui.increaseActiveWorkers()
	runDrawsSince(ui, n)
	require.True(t, ui.statusOverlay.visible)
	assert.Contains(t, ui.statusOverlay.status.GetText(true), "Background deletions: 1")

	n = drawsQueued(ui)
	ui.decreaseActiveWorkers()
	runDrawsSince(ui, n)
	// the popup lingers briefly so a short deletion is still visible
	assert.True(t, ui.statusOverlay.visible)

	// the delayed update hides it once the linger has passed
	ui.updateDeletionStatus()
	assert.False(t, ui.statusOverlay.visible)
}

func TestQueuedBackgroundDeletionIsDimmed(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	require.NoError(t, os.MkdirAll("test_dir/other", 0o755))

	ui := getAnalyzedPathMockedApp(t, true, true, false)
	ui.SetResultRowDirectoryColor("#3498db")
	ui.done = make(chan struct{})
	ui.deleteWorkersCount = 1

	started := make(chan struct{})
	proceed := make(chan struct{})
	var once sync.Once
	ui.newDeleter = fakeDeleter(func(dir, item fs.Item) error {
		once.Do(func() { close(started) })
		<-proceed
		return nil
	})
	ui.SetDeleteInBackground()

	ui.table.Select(rowOf(t, ui, "nested"), 0)
	deleteNow(ui, ActionDelete)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first deletion did not start")
	}

	// the only worker is busy, so this one stays queued
	ui.table.Select(rowOf(t, ui, "other"), 0)
	deleteNow(ui, ActionDelete)

	nestedRow := rowOf(t, ui, "nested")
	otherRow := rowOf(t, ui, "other")
	// the item being deleted is dimmed as well
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, nestedRow))
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, otherRow))
	// the directory colour tag must not override the queued cell colour
	assert.NotContains(t, ui.table.GetCell(otherRow, 0).Text, "#3498db")
	selectedFg, _, _ := ui.table.GetCell(otherRow, 0).SelectedStyle.Decompose()
	assert.Equal(t, ui.queuedTextColor, selectedFg)

	// a redraw must keep the active and the queued item dimmed
	ui.refreshListing()
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, nestedRow))
	assert.Equal(t, ui.queuedTextColor, cellForeground(ui, otherRow))

	close(proceed)
	waitDone(t, ui)
	waitDone(t, ui)
}

func TestBackgroundDeletionShowsStatusPopup(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	ui := getAnalyzedPathMockedApp(t, false, true, false)
	ui.done = make(chan struct{})
	started := make(chan struct{})
	proceed := make(chan struct{})
	ui.newDeleter = fakeDeleter(func(dir, item fs.Item) error {
		close(started)
		<-proceed
		return nil
	})
	ui.SetDeleteInBackground()

	waitForDraw := func(after int) {
		t.Helper()
		require.Eventually(t, func() bool {
			return drawsQueued(ui) > after
		}, 5*time.Second, 10*time.Millisecond)
		runDrawsSince(ui, after)
	}

	n := drawsQueued(ui)
	ui.table.Select(0, 0)
	deleteNow(ui, ActionDelete)

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the deletion did not start")
	}

	waitForDraw(n)
	require.True(t, ui.statusOverlay.visible)
	assert.Contains(t, ui.statusOverlay.status.GetText(true), "Background deletions: 1")

	close(proceed)
	waitDone(t, ui)

	require.Eventually(t, func() bool {
		runDrawsSince(ui, n)
		return !ui.statusOverlay.visible
	}, 5*time.Second, 10*time.Millisecond)
}
