package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// snapshotFixture scans a real tree, then swaps the "snap" dir for an empty placeholder
// the way the analyzers do for skipped btrfs subvolumes.
func snapshotFixture(t *testing.T) (ui *UI, root *analyze.Dir, snapPath string) {
	t.Helper()
	ui = newTestUI()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), make([]byte, 4096), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "snap"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "snap", "inner.txt"), make([]byte, 8192), 0o600))
	scan(t, ui, dir)

	root = ui.topDir.(*analyze.Dir)
	for index, file := range root.Files {
		if file.GetName() == "snap" {
			root.Files[index] = &analyze.Dir{File: &analyze.File{Name: "snap", Flag: 'S', Parent: root}}
		}
	}
	root.UpdateStats(make(fs.HardLinkedItems))
	return ui, root, filepath.Join(dir, "snap")
}

func TestOpeningUnscannedSnapshotScansIt(t *testing.T) {
	ui, root, snapPath := snapshotFixture(t)
	usageBefore := root.GetUsage()

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape(snapPath))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	node, err := ui.findNode(snapPath)
	require.NoError(t, err)
	assert.False(t, analyze.IsUnscannedSnapshot(node))
	assert.Equal(t, 'S', node.GetFlag())
	assert.Greater(t, node.GetUsage(), int64(0))
	_, found := childByName(node, "inner.txt")
	assert.True(t, found)

	root.UpdateStats(make(fs.HardLinkedItems))
	assert.Equal(t, usageBefore, root.GetUsage(), "scope must not count toward the parent")
}

func TestDeleteRefusedInsideSnapshotScope(t *testing.T) {
	ui, _, snapPath := snapshotFixture(t)
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape(snapPath))
	require.NoError(t, err)
	resp.Body.Close()

	for _, path := range []string{snapPath, filepath.Join(snapPath, "inner.txt")} {
		req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
		require.NoError(t, err)
		req.Header.Set("X-GDU-Action", ui.actionToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, path)
		_, err = os.Stat(path)
		assert.NoError(t, err, "snapshot content must stay on disk")
	}
}
