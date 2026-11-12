package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createSnapshotUI builds a tree by hand: a plain file plus an unscanned snapshot placeholder
// that points at a real directory holding one file.
func createSnapshotUI(t *testing.T) (*UI, *analyze.Dir, *analyze.Dir) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "snap"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "snap", "inside"), make([]byte, 8192), 0o644))

	top := &analyze.Dir{
		File:     &analyze.File{Name: filepath.Base(root)},
		BasePath: filepath.Dir(root),
		Files:    fs.Files{},
	}
	top.AddFile(&analyze.File{Name: "plain", Size: 4096, Usage: 4096, Parent: top})
	placeholder := &analyze.Dir{
		File:  &analyze.File{Name: "snap", Flag: 'S', Parent: top},
		Files: fs.Files{},
	}
	top.AddFile(placeholder)
	top.UpdateStats(make(fs.HardLinkedItems))

	simScreen := testapp.CreateSimScreen()
	t.Cleanup(simScreen.Fini)
	ui := CreateUI(testapp.CreateMockedApp(true), simScreen, &bytes.Buffer{}, false, true, false, false)
	ui.done = make(chan struct{})
	ui.topDir = top
	ui.topDirPath = top.GetPath()
	ui.currentDir = top
	ui.showDir()
	return ui, top, placeholder
}

func selectRowOf(t *testing.T, ui *UI, name string) int {
	t.Helper()
	for row := 0; row < ui.table.GetRowCount(); row++ {
		if item, ok := ui.table.GetCell(row, 0).GetReference().(fs.Item); ok && item.GetName() == name {
			ui.table.Select(row, 0)
			return row
		}
	}
	t.Fatalf("row %s not found", name)
	return -1
}

func TestEnterUnscannedSnapshotScansScope(t *testing.T) {
	ui, top, placeholder := createSnapshotUI(t)
	sizeBefore := top.GetUsage()
	require.True(t, analyze.IsUnscannedSnapshot(placeholder))

	row := selectRowOf(t, ui, "snap")
	ui.fileItemSelected(row, 0)
	<-ui.done
	for _, draw := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		draw()
	}

	assert.Equal(t, "snap", ui.currentDir.GetName())
	assert.NotSame(t, placeholder, ui.currentDir)
	assert.Equal(t, rune('S'), ui.currentDir.GetFlag())
	assert.Greater(t, ui.currentDir.GetUsage(), int64(0))
	assert.False(t, analyze.IsUnscannedSnapshot(ui.currentDir))
	assert.Same(t, top, ui.currentDir.GetParent())
	assert.False(t, ui.pages.HasPage("progress"))

	// parent total is unchanged and the scope is in the tree
	assert.Equal(t, sizeBefore, top.GetUsage())
	found := false
	for item := range top.GetFiles(fs.SortByName, fs.SortAsc) {
		found = found || item == ui.currentDir
	}
	assert.True(t, found)

	// going back up lands on the parent with the scope row selected
	ui.handleLeft()
	assert.Same(t, top, ui.currentDir)
	assert.Equal(t, sizeBefore, ui.currentDir.GetUsage())
}

func TestDeleteRefusedInSnapshotScope(t *testing.T) {
	ui, _, _ := createSnapshotUI(t)
	ui.fileItemSelected(selectRowOf(t, ui, "snap"), 0)
	<-ui.done
	for _, draw := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		draw()
	}

	// inside the scope
	selectRowOf(t, ui, "inside")
	ui.askBeforeDelete = false
	ui.handleDelete(ActionDelete)
	assert.Contains(t, ui.header.GetText(false), "read-only")
	assert.False(t, ui.pages.HasPage("deleting"))

	// marked items
	ui.header.Clear()
	ui.markedRows[1] = struct{}{}
	ui.handleDelete(ActionEmpty)
	assert.Contains(t, ui.header.GetText(false), "read-only")
	assert.FileExists(t, filepath.Join(ui.currentDir.GetPath(), "inside"))
}

func TestDeleteRefusedOnSnapshotRow(t *testing.T) {
	ui, _, _ := createSnapshotUI(t)
	selectRowOf(t, ui, "snap")
	ui.askBeforeDelete = true
	ui.handleDelete(ActionDelete)
	assert.Contains(t, ui.header.GetText(false), "read-only")
	assert.False(t, ui.pages.HasPage("confirm"))
}
