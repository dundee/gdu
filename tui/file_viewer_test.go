package tui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
)

type viewerApplication struct {
	common.TermApplication
	suspended bool
	restored  bool
	calls     int
}

func (app *viewerApplication) Suspend(f func()) bool {
	app.calls++
	app.suspended = true
	defer func() {
		app.suspended = false
		app.restored = true
	}()
	f()
	return true
}

func createFileViewerUI(t *testing.T) (*UI, *viewerApplication, string) {
	t.Helper()
	root := t.TempDir()
	name := "space Ω;$(literal)&.txt"
	path := filepath.Join(root, name)
	require.NoError(t, os.WriteFile(path, []byte("viewer fixture sentinel\n"), 0o600))
	dir := &analyze.Dir{File: &analyze.File{Name: filepath.Base(root)}, BasePath: filepath.Dir(root)}
	file := &analyze.File{Name: name, Parent: dir}
	screen := testapp.CreateSimScreen()
	t.Cleanup(screen.Fini)
	app := &viewerApplication{TermApplication: testapp.CreateMockedApp(false)}
	ui := CreateUI(app, screen, &bytes.Buffer{}, false, true, false, false)
	ui.currentDir = dir
	ui.currentDirPath = root
	ui.table.SetCell(0, 0, tview.NewTableCell(name).SetReference(file))
	ui.table.Select(0, 0)
	return ui, app, path
}

func TestFileViewerLiteralArgumentAndTerminalRestore(t *testing.T) {
	for _, failure := range []error{nil, os.ErrNotExist, errors.New("exit status 7")} {
		name := "success"
		if failure != nil {
			name = failure.Error()
		}
		t.Run(name, func(t *testing.T) {
			ui, app, path := createFileViewerUI(t)
			command := filepath.Join(t.TempDir(), "viewer executable")
			ui.SetFileViewer(command)
			t.Setenv("GDU_VIEWER_TEST", "literal")
			calls := 0
			ui.exec = func(name string, args, env []string) error {
				calls++
				assert.True(t, app.suspended)
				assert.Equal(t, command, name)
				assert.Equal(t, []string{path}, args)
				assert.Contains(t, env, "GDU_VIEWER_TEST=literal")
				return failure
			}
			ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'v', 0))
			assert.Equal(t, 1, calls)
			assert.Equal(t, 1, app.calls)
			assert.True(t, app.restored)
			assert.False(t, app.suspended)
			assert.False(t, ui.pages.HasPage("file"))
			if failure != nil {
				assert.True(t, ui.pages.HasPage("error"))
			} else {
				assert.False(t, ui.pages.HasPage("error"))
			}
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, "viewer fixture sentinel\n", string(data))
		})
	}
}

func TestFileViewerRespectsDisabledAndSyntheticRows(t *testing.T) {
	cases := map[string]func(*UI){
		"no view":      func(ui *UI) { ui.SetNoViewFile() },
		"no spawn":     func(ui *UI) { ui.SetNoSpawnShell() },
		"preview":      func(ui *UI) { ui.previewing = true },
		"archive":      func(ui *UI) { ui.currentDir = &analyze.ZipDir{Dir: ui.currentDir.(*analyze.Dir)} },
		"virtual root": func(ui *UI) { ui.currentDir = analyze.CreateVirtualRootDir() },
		"parent row":   func(ui *UI) { ui.table.GetCell(0, 0).SetReference(ui.currentDir) },
		"empty row":    func(ui *UI) { ui.table.GetCell(0, 0).SetReference(nil) },
	}
	for name, configure := range cases {
		t.Run(name, func(t *testing.T) {
			ui, app, _ := createFileViewerUI(t)
			ui.SetFileViewer("never-executed-viewer")
			ui.exec = func(string, []string, []string) error {
				t.Fatal("external viewer executed for a disabled or synthetic row")
				return nil
			}
			configure(ui)
			ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'v', 0))
			assert.Zero(t, app.calls)
			assert.False(t, ui.pages.HasPage("file"))
		})
	}
}

func TestFileViewerMouseDoubleClick(t *testing.T) {
	ui, app, path := createFileViewerUI(t)
	command := filepath.Join(t.TempDir(), "mouse viewer executable")
	ui.SetFileViewer(command)
	// Row zero is the parent row. Select the real file on row one so the
	// production mouse handler takes the file branch, not parent navigation.
	ui.table.SetCell(1, 0, ui.table.GetCell(0, 0))
	ui.table.SetCell(0, 0, tview.NewTableCell("..").SetReference(ui.currentDir))
	ui.table.Select(1, 0)
	calls := 0
	ui.exec = func(name string, args, _ []string) error {
		calls++
		assert.True(t, app.suspended)
		assert.Equal(t, command, name)
		assert.Equal(t, []string{path}, args)
		return nil
	}

	event, action := ui.onMouse(tcell.NewEventMouse(0, 0, tcell.Button1, 0), tview.MouseLeftDoubleClick)
	assert.Nil(t, event)
	assert.Equal(t, tview.MouseLeftDoubleClick, action)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 1, app.calls)
	assert.True(t, app.restored)
	assert.False(t, app.suspended)
	assert.False(t, ui.pages.HasPage("file"))
	assert.False(t, ui.pages.HasPage("error"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "viewer fixture sentinel\n", string(data))
}

func TestFileViewerUnsetKeepsBuiltInWithNoSpawn(t *testing.T) {
	ui, app, _ := createFileViewerUI(t)
	ui.SetNoSpawnShell()
	ui.exec = func(string, []string, []string) error {
		t.Fatal("default viewer must not spawn a process")
		return nil
	}
	view := ui.showFile()
	require.NotNil(t, view)
	assert.True(t, ui.pages.HasPage("file"))
	assert.Zero(t, app.calls)
	view.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'q', 0))
	assert.False(t, ui.pages.HasPage("file"))
}
