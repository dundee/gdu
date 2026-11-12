package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/pkg/remove"
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
