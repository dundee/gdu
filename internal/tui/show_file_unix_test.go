//go:build !windows && !plan9

package tui

import (
	"bytes"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/testapp"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/fs"
)

// Viewing a named pipe used to block the UI until a writer came.
func TestViewingAFIFODoesNotHang(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600))
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()
	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})
	ui.done = make(chan struct{})
	require.NoError(t, ui.AnalyzePath(dir, nil))
	<-ui.done
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}
	for row := range ui.table.GetRowCount() {
		if item, ok := ui.table.GetCell(row, 0).GetReference().(fs.Item); ok && item.GetName() == "pipe" {
			ui.table.Select(row, 0)
		}
	}

	shown := make(chan bool, 1)
	go func() { shown <- ui.showFile() != nil }()

	select {
	case viewed := <-shown:
		assert.False(t, viewed)
		assert.False(t, ui.pages.HasPage("file"))
	case <-time.After(5 * time.Second):
		t.Fatal("viewing a FIFO hangs")
	}
}
