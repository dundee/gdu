package tui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/testapp"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/stretchr/testify/require"
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
	ui := CreateUI(app, screen, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Colors: true, ApparentSize: true}})
	ui.done = make(chan struct{})
	require.NoError(t, ui.AnalyzePath(root, nil))
	<-ui.done
	for _, draw := range app.GetUpdateDraws() {
		draw()
	}
	ui.setDeleteInBackground()

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
