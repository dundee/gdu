package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// Deletion in the background changes the tree while the user goes on
// browsing it, and an export may be encoding it. Run with -race.
func TestBackgroundDeletionWhileBrowsingAndExporting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	for i := range 60 {
		dir := filepath.Join(root, fmt.Sprintf("d%02d", i), "nested")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		for j := range 3 {
			require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprint(j)), []byte("data"), 0o600))
		}
	}

	screen := testapp.CreateSimScreen()
	defer screen.Fini()
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, screen, &bytes.Buffer{}, true, true, false, false)
	ui.done = make(chan struct{})
	require.NoError(t, ui.AnalyzePath(root, nil))
	<-ui.done
	for _, draw := range app.GetUpdateDraws() {
		draw()
	}
	ui.SetDeleteInBackground()

	top := ui.currentDir
	var targets []deleteTarget
	for item := range ui.session.Root().GetFiles(fs.SortByName, fs.SortAsc) {
		targets = append(targets, deleteTarget{dir: top, item: item})
	}
	require.Len(t, targets, 60)

	deleted := make(chan struct{})
	go func() {
		for range targets {
			<-ui.done
		}
		close(deleted)
	}()
	var export sync.WaitGroup
	export.Go(func() {
		for {
			select {
			case <-deleted:
				return
			default:
				_ = ui.exportTree(io.Discard)
			}
		}
	})

	ui.queueForDeletion(targets, ActionDelete, ui.deleter())
	for browsing := true; browsing; {
		select {
		case <-deleted:
			browsing = false
		default:
			ui.showDir()
		}
	}
	export.Wait()

	ui.showDir()
	require.Equal(t, 0, ui.table.GetRowCount(), "every entry is gone")
}
