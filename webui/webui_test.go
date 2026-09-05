package webui

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func newTestUI() *UI {
	return CreateUI(io.Discard, "127.0.0.1:0", false, "", false, false, false, false)
}

// makeTree creates a temporary directory tree and returns its root path.
func makeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(root, "big.bin"), make([]byte, 4096), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "small.txt"), make([]byte, 16), 0o600))
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(sub, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "nested.dat"), make([]byte, 1024), 0o600))
	return root
}

func scan(t *testing.T, ui *UI, path string) {
	t.Helper()
	require.NoError(t, ui.AnalyzePath(path, nil))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ui.buildStatus().State == "done" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan did not complete in time")
}

func TestStatusEndpoint(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	var status statusResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&status))
	assert.Equal(t, "done", status.State)
	assert.Equal(t, root, status.RootPath)
	assert.True(t, status.DeleteAllowed, "delete should be allowed for an unfiltered local scan")
}

func TestNodesEndpoint(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape(root))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var node nodeResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&node))

	assert.True(t, node.Node.IsDir, "root node should be a directory")
	assert.Len(t, node.Breadcrumbs, 1)
	require.Len(t, node.Children, 3)
	// Default sort is by disk usage descending. Assert the returned order is
	// monotonically non-increasing rather than depending on fixture specifics
	// (sparse files can have near-zero disk usage regardless of apparent size).
	for i := 1; i < len(node.Children); i++ {
		assert.GreaterOrEqualf(t, node.Children[i-1].Usage, node.Children[i].Usage,
			"children not sorted by usage desc: %+v", node.Children)
	}
	for _, c := range node.Children {
		assert.Greaterf(t, c.Size, int64(0), "child %q has non-positive size", c.Name)
	}
}

func TestNodesNested(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	sub := filepath.Join(root, "sub")
	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape(sub))
	require.NoError(t, err)
	defer resp.Body.Close()

	var node nodeResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&node))
	assert.Equal(t, "sub", node.Node.Name)
	assert.Len(t, node.Breadcrumbs, 2)
	if assert.Len(t, node.Children, 1) {
		assert.Equal(t, "nested.dat", node.Children[0].Name)
	}
}

type treePayload struct {
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	IsDir     bool          `json:"isDir"`
	Size      int64         `json:"size"`
	Usage     int64         `json:"usage"`
	ItemCount int64         `json:"itemCount"`
	Children  []treePayload `json:"children"`
}

func TestTreeEndpointReturnsCompleteSubtree(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/tree?path=" + url.QueryEscape(root))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tree treePayload
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tree))
	require.Equal(t, root, tree.Path)
	require.True(t, tree.IsDir)
	require.Greater(t, tree.Size, int64(0), "tree root is missing size metadata: %+v", tree)
	require.Greater(t, tree.Usage, int64(0), "tree root is missing size metadata: %+v", tree)
	require.Greater(t, tree.ItemCount, int64(0), "tree root is missing size metadata: %+v", tree)

	var sub *treePayload
	for i := range tree.Children {
		if tree.Children[i].Name == "sub" {
			sub = &tree.Children[i]
			break
		}
	}
	require.NotNil(t, sub, "sub directory missing from tree")
	if assert.Len(t, sub.Children, 1) {
		assert.Equal(t, "nested.dat", sub.Children[0].Name)
		assert.NotNil(t, sub.Children[0].Children)
	}
}

func TestTreeEndpointRejectsPathOutsideRoot(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/tree?path=" + url.QueryEscape(filepath.Dir(root)))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestNodesPathTraversalRejected(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape("/etc"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestNodesNotFound(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	missing := filepath.Join(root, "does-not-exist")
	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape(missing))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteNodeEndpointRemovesFileAndUpdatesTree(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "deleted file still exists: %v", err)

	_, err = ui.findNode(path)
	assert.ErrorIs(t, err, errNotFound)
}

func TestDeleteNodeEndpointTrashModeUsesTrasher(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	var trashedPath string
	ui.trasher = func(dir, item fs.Item) error {
		trashedPath = item.GetPath()
		dir.RemoveFile(item)
		return nil
	}

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(
		http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path)+"&mode=trash", nil,
	)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, path, trashedPath)
	// The trasher stub above does not touch the filesystem, unlike a
	// permanent delete: the file being untouched on disk confirms the
	// permanent-delete codepath (remove.ItemFromDir) was not used instead.
	_, err = os.Stat(path)
	assert.NoError(t, err, "trash mode must not permanently delete the file")

	_, err = ui.findNode(path)
	assert.ErrorIs(t, err, errNotFound)
}

func TestDeleteNodeEndpointReturnsErrorWhenRemoveFuncFails(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	ui.trasher = func(dir fs.Item, item fs.Item) error {
		return errors.New("trash command failed")
	}

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(
		http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path)+"&mode=trash", nil,
	)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	_, err = os.Stat(path)
	assert.NoError(t, err, "file must still exist after a failed delete")
}

func TestRevealEndpointOpensParentDirectoryForFile(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	var revealed string
	ui.revealPath = func(path string) error {
		revealed = path
		return nil
	}
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	body := strings.NewReader(fmt.Sprintf(`{"path":%q}`, filepath.Join(root, "big.bin")))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reveal", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, root, revealed)
}

// makeZip creates a zip archive at path containing a single file entry
// "inner.txt" and returns the temp directory it lives in.
func makeZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create("inner.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
}

// makeNestedZip creates a zip archive at path containing a single file entry
// two levels deep ("subdir/deep.txt") and returns the temp directory it
// lives in.
func makeNestedZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create("subdir/deep.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
}

func TestRevealEndpointOpensArchiveParentDirectoryForDeeplyNestedEntry(t *testing.T) {
	ui := newTestUI()
	ui.SetArchiveBrowsing(true)
	root := t.TempDir()
	zipPath := filepath.Join(root, "archive.zip")
	makeNestedZip(t, zipPath)
	scan(t, ui, root)

	var revealed string
	ui.revealPath = func(path string) error {
		revealed = path
		return nil
	}
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	// deep.txt's parent (subdir) is itself nested inside the archive root,
	// so realPathAncestor must walk up two levels before reaching the
	// archive's own top-level node.
	body := strings.NewReader(fmt.Sprintf(`{"path":%q}`, filepath.Join(zipPath, "subdir", "deep.txt")))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reveal", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, root, revealed)
}

func TestRevealEndpointOpensArchiveParentDirectoryForNestedEntry(t *testing.T) {
	ui := newTestUI()
	ui.SetArchiveBrowsing(true)
	root := t.TempDir()
	zipPath := filepath.Join(root, "archive.zip")
	makeZip(t, zipPath)
	scan(t, ui, root)

	var revealed string
	ui.revealPath = func(path string) error {
		revealed = path
		return nil
	}
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	body := strings.NewReader(fmt.Sprintf(`{"path":%q}`, filepath.Join(zipPath, "inner.txt")))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reveal", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, root, revealed, "revealed path should be the archive's containing directory, not the archive file itself")
}

func TestSetTrashCommandReplacesDefaultTrasher(t *testing.T) {
	ui := newTestUI()
	before := reflect.ValueOf(ui.trasher).Pointer()

	ui.SetTrashCommand("true")

	after := reflect.ValueOf(ui.trasher).Pointer()
	assert.NotEqual(t, before, after, "SetTrashCommand should replace the default trasher")
}

func TestDeleteEndpointHonorsNoDelete(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)
	ui.SetNoDelete()

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_, err = os.Stat(path)
	assert.NoError(t, err, "file should remain after rejected delete")
}

func TestDeleteEndpointHonorsActiveFilters(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)
	ui.FilteringFiles = true
	t.Setenv("GDU_ALLOW_DELETE_WITH_FILTER", "")

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_, err = os.Stat(path)
	assert.NoError(t, err, "file should remain after rejected delete")
}

func TestDeleteEndpointRejectsAnalysisRoot(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(root), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_, err = os.Stat(root)
	assert.NoError(t, err, "root should remain after rejected delete")
}

func TestActionEndpointsRequireLocalActionHeader(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_, err = os.Stat(path)
	assert.NoError(t, err, "file should remain after rejected delete")
}

func TestActionEndpointsRejectWrongToken(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", "not-the-real-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_, err = os.Stat(path)
	assert.NoError(t, err, "file should remain after rejected delete")
}

func TestActionEndpointsRejectCrossSiteFetch(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_, err = os.Stat(path)
	assert.NoError(t, err, "file should remain after rejected delete")
}

func TestActionEndpointsAllowSameOriginFetchMetadata(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestActionEndpointsRejectMismatchedOrigin(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	req.Header.Set("Origin", "http://attacker.example")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_, err = os.Stat(path)
	assert.NoError(t, err, "file should remain after rejected delete")
}

func TestActionEndpointsAllowMatchingOrigin(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()
	path := filepath.Join(root, "big.bin")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	req.Header.Set("Origin", "http://"+req.URL.Host)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestParseSort(t *testing.T) {
	cases := []struct {
		sort, order string
		wantBy      fs.SortBy
		wantOrder   fs.SortOrder
	}{
		{"", "", fs.SortBySize, fs.SortDesc},
		{"name", "asc", fs.SortByName, fs.SortAsc},
		{"itemCount", "desc", fs.SortByItemCount, fs.SortDesc},
		{"mtime", "asc", fs.SortByMtime, fs.SortAsc},
		{"size", "asc", fs.SortBySize, fs.SortAsc},
	}
	for _, c := range cases {
		gotBy, gotOrder := parseSort(c.sort, c.order)
		assert.Equalf(t, c.wantBy, gotBy, "parseSort(%q,%q) by", c.sort, c.order)
		assert.Equalf(t, c.wantOrder, gotOrder, "parseSort(%q,%q) order", c.sort, c.order)
	}
}

func TestBrowserCommand(t *testing.T) {
	cases := []struct {
		goos, cmd string
		wantName  string
		wantArgs  []string
	}{
		{"darwin", "", "open", []string{"http://x"}},
		{"windows", "", "rundll32", []string{"url.dll,FileProtocolHandler", "http://x"}},
		{"linux", "", "xdg-open", []string{"http://x"}},
		{"linux", "firefox", "firefox", []string{"http://x"}},
		{"linux", "flatpak run org.mozilla.firefox", "flatpak", []string{"run", "org.mozilla.firefox", "http://x"}},
	}
	for _, c := range cases {
		name, args, err := browserCommand(c.goos, "http://x", c.cmd)
		require.NoError(t, err)
		assert.Equal(t, c.wantName, name)
		assert.Equal(t, c.wantArgs, args)
	}
}

func TestSetCollapsePath(t *testing.T) {
	ui := newTestUI()
	require.False(t, ui.collapsePath, "collapsePath should default to false")
	ui.SetCollapsePath(true)
	assert.True(t, ui.collapsePath, "SetCollapsePath(true) did not set the field")
}

func TestCreateUIDefaultListenAddr(t *testing.T) {
	ui := CreateUI(io.Discard, "", false, "", true, true, true, true)
	assert.Equal(t, "localhost:0", ui.listenAddr)
	assert.True(t, ui.UseColors && ui.ShowApparentSize && ui.ShowRelativeSize && ui.UseSIPrefix,
		"display options not propagated to embedded common.UI")
}

func TestListDevicesAndHandleDevices(t *testing.T) {
	ui := newTestUI()

	devices := device.Devices{
		&device.Device{
			Name:       "/dev/sda1",
			MountPoint: "/",
			Fstype:     "ext4",
			Size:       1000,
			Free:       400,
		},
	}
	getter := testdev.DevicesInfoGetterMock{Devices: devices}

	require.NoError(t, ui.ListDevices(getter))

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/devices")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out []deviceJSON
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Len(t, out, 1)
	d := out[0]
	assert.Equal(t, "/dev/sda1", d.Name)
	assert.Equal(t, "/", d.MountPoint)
	assert.Equal(t, "ext4", d.Fstype)
	assert.EqualValues(t, 1000, d.Size)
	assert.EqualValues(t, 400, d.Free)
}

func TestHandleDevicesEmpty(t *testing.T) {
	ui := newTestUI()

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/devices")
	require.NoError(t, err)
	defer resp.Body.Close()

	var out []deviceJSON
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Empty(t, out)
}

const sampleReport = `
	[1,2,{"progname":"gdu","progver":"development","timestamp":1626806293},
	[{"name":"/home/xxx","mtime":1629333600},
	{"name":"gdu.json","asize":33805233,"dsize":33808384},
	[{"name":"app"},
	{"name":"app.go","asize":4638,"dsize":8192},
	{"name":"app_test.go","asize":4974,"dsize":8192}],
	{"name":"main.go","asize":3205,"dsize":4096,"mtime":1629333600}]]
`

func TestReadAnalysis(t *testing.T) {
	ui := newTestUI()

	require.NoError(t, ui.ReadAnalysis(strings.NewReader(sampleReport)))

	status := ui.buildStatus()
	assert.Equal(t, "done", status.State)
	assert.Equal(t, "/home/xxx", status.RootPath)

	node, err := ui.findNode("/home/xxx")
	require.NoError(t, err)
	assert.Equal(t, "xxx", node.GetName())
	assert.True(t, node.IsDir(), "root should be a directory")
}

func TestReadAnalysisError(t *testing.T) {
	ui := newTestUI()

	err := ui.ReadAnalysis(strings.NewReader("not valid json"))
	require.Error(t, err, "expected error for invalid input")

	status := ui.buildStatus()
	assert.Equal(t, "error", status.State)
	assert.NotEmpty(t, status.Error, "expected non-empty error message in status")
	assert.False(t, ui.scanning, "scanning flag should be cleared after error")
}

func TestReadFromStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	storagePath := t.TempDir() + "/badger"

	// Populate the storage with a stored analysis.
	a := analyze.CreateStoredAnalyzer(storagePath)
	dir := a.AnalyzeDir(
		"test_dir",
		func(_, _ string) bool { return false },
		func(_ string) bool { return false },
	)
	a.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	ui := newTestUI()
	require.NoError(t, ui.ReadFromStorage(storagePath, "test_dir"))

	status := ui.buildStatus()
	assert.Equal(t, "done", status.State)

	node, err := ui.findNode("test_dir")
	require.NoError(t, err)
	assert.Equal(t, "test_dir", node.GetName())
	assert.EqualValues(t, 5, node.GetItemCount())
}

func TestReadFromStorageError(t *testing.T) {
	ui := newTestUI()
	// A non-existent path yields a badger read error.
	err := ui.ReadFromStorage(t.TempDir()+"/badger", "missing_dir")
	assert.Error(t, err, "expected error reading a missing dir from storage")
}

func TestBuildStatusStates(t *testing.T) {
	// Fresh UI: no topDir yet, not scanning -> reported as scanning.
	ui := newTestUI()
	assert.Equal(t, "scanning", ui.buildStatus().State)

	// Explicit scanning flag.
	ui.mu.Lock()
	ui.scanning = true
	ui.mu.Unlock()
	assert.Equal(t, "scanning", ui.buildStatus().State)
}

func TestStatusJSON(t *testing.T) {
	ui := newTestUI()
	ui.mu.Lock()
	ui.topDirPath = "/tmp/root"
	ui.progress = common.CurrentProgress{
		CurrentItemName: "current",
		ItemCount:       3,
		TotalUsage:      100,
	}
	ui.mu.Unlock()

	raw := ui.statusJSON()

	var status statusResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &status), "statusJSON produced invalid JSON")
	assert.Equal(t, "/tmp/root", status.RootPath)
	assert.EqualValues(t, 3, status.Progress.ItemCount)
	assert.EqualValues(t, 100, status.Progress.TotalUsage)
	assert.Equal(t, "current", status.Progress.CurrentItem)
}

func TestHandleEventsSSE(t *testing.T) {
	ui := newTestUI()
	ui.mu.Lock()
	ui.topDirPath = "/tmp/root"
	ui.mu.Unlock()

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/events", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// The initial state is pushed immediately on connect.
	reader := bufio.NewReader(resp.Body)
	line, err := readSSEData(t, reader)
	require.NoError(t, err, "reading initial SSE frame")
	var status statusResponse
	require.NoErrorf(t, json.Unmarshal([]byte(line), &status), "initial SSE frame not valid JSON (%q)", line)
	assert.Equal(t, "/tmp/root", status.RootPath)

	// A subsequent publish is delivered to the connected subscriber.
	ui.hub.publish(`{"state":"custom"}`)
	line, err = readSSEData(t, reader)
	require.NoError(t, err, "reading published SSE frame")
	assert.Equal(t, `{"state":"custom"}`, line)
}

// readSSEData reads lines until it finds a `data: ` frame and returns its payload.
func readSSEData(t *testing.T, r *bufio.Reader) (string, error) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: "), nil
		}
	}
	return "", io.EOF
}

func TestHub(t *testing.T) {
	h := newHub()

	ch, last := h.subscribe()
	assert.Empty(t, last, "last should be empty before any publish")

	h.publish("hello")
	select {
	case msg := <-ch:
		assert.Equal(t, "hello", msg)
	case <-time.After(time.Second):
		t.Fatal("did not receive published message")
	}

	// A late subscriber gets the most recent message via `last`.
	ch2, last2 := h.subscribe()
	assert.Equal(t, "hello", last2)

	h.unsubscribe(ch)
	// unsubscribe closes the channel.
	_, open := <-ch
	assert.False(t, open, "channel should be closed after unsubscribe")

	// Unsubscribing twice is a no-op and must not panic.
	h.unsubscribe(ch)

	h.unsubscribe(ch2)
}

func TestHubPublishDoesNotBlockOnFullSubscriber(t *testing.T) {
	h := newHub()
	ch, _ := h.subscribe()
	defer h.unsubscribe(ch)

	// The subscriber channel is buffered to 8; publishing more than the buffer
	// must not block because publish drops on a full channel.
	for i := 0; i < 100; i++ {
		h.publish("msg")
	}
}

func TestWarnIfRemote(t *testing.T) {
	cases := []struct {
		name     string
		addr     net.Addr
		wantWarn bool
	}{
		{
			name:     "loopback ipv4",
			addr:     &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
			wantWarn: false,
		},
		{
			name:     "loopback ipv6",
			addr:     &net.TCPAddr{IP: net.ParseIP("::1"), Port: 8080},
			wantWarn: false,
		},
		{
			name:     "remote",
			addr:     &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 8080},
			wantWarn: true,
		},
		{
			name:     "lan",
			addr:     &net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 8080},
			wantWarn: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			warnIfRemote(&buf, c.addr)
			warned := strings.Contains(buf.String(), "WARNING")
			assert.Equalf(t, c.wantWarn, warned, "output: %q", buf.String())
		})
	}
}

func TestToNodeJSONFlag(t *testing.T) {
	parent := &analyze.Dir{File: &analyze.File{Name: "root"}}

	// A file with a meaningful flag exposes it in the JSON payload.
	flagged := &analyze.File{Name: "denied", Flag: '!', Parent: parent}
	assert.Equal(t, "!", toNodeJSON(flagged).Flag)

	// A blank flag (space) is omitted.
	blank := &analyze.File{Name: "ok", Flag: ' ', Parent: parent}
	assert.Empty(t, toNodeJSON(blank).Flag)

	// A zero flag is also omitted.
	zero := &analyze.File{Name: "zero", Parent: parent}
	assert.Empty(t, toNodeJSON(zero).Flag)
}

// TestToNodeJSONItemCount pins the wire format to the displayed item count, so
// the browser shows the same number the API sorts by.
func TestToNodeJSONItemCount(t *testing.T) {
	parent := &analyze.Dir{File: &analyze.File{Name: "root"}}

	// A directory reports what it contains, not counting itself.
	dir := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: parent}, ItemCount: 4}
	if got := toNodeJSON(dir).ItemCount; got != 3 {
		t.Errorf("dir itemCount = %d, want 3", got)
	}

	// A file counts itself.
	file := &analyze.File{Name: "leaf", Parent: parent}
	if got := toNodeJSON(file).ItemCount; got != 1 {
		t.Errorf("file itemCount = %d, want 1", got)
	}
}

func TestAnalyzePathWithParent(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)

	// First scan the root so we have a parent dir in the tree.
	scan(t, ui, root)

	parent, err := ui.findNode(root)
	require.NoError(t, err)

	// Re-scan the "sub" directory into the existing parent. This exercises the
	// parentDir != nil branch: SetParent / RemoveFileByName / AddFile. The
	// re-scanned dir replaces the old child in the parent and keeps the same
	// top-level tree root.
	sub := filepath.Join(root, "sub")
	require.NoError(t, ui.AnalyzePath(sub, parent))
	waitDone(t, ui)

	// The refreshed "sub" child is present in the parent and points back to it.
	child, found := childByName(parent, "sub")
	require.True(t, found, "re-scanned sub not found under parent")
	assert.Equal(t, "sub", child.GetName())
	assert.Equal(t, parent, child.GetParent(), "re-scanned sub should have its parent set to the original root")
	assert.GreaterOrEqual(t, child.GetItemCount(), int64(1), "re-scanned sub should have counted its files")
}

func waitDone(t *testing.T, ui *UI) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ui.buildStatus().State == "done" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan did not complete in time")
}

func TestHandleStatusHidesDeleteAllowedForRemoteRequest(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.RemoteAddr = "203.0.113.5:12345"
	req.Host = "example.com"
	w := httptest.NewRecorder()
	ui.handleStatus(w, req)

	var status statusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&status))
	assert.False(t, status.DeleteAllowed, "remote requests must never see delete as allowed")
}

func TestNodesEndpointDefaultsToRootWhenPathOmitted(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/nodes")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var node nodeResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&node))
	assert.Equal(t, root, node.Node.Path)
}

func TestNodesPathThroughFileSegmentNotFound(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	bogus := filepath.Join(root, "big.bin", "nested")
	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape(bogus))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestFindNodeBeforeScan(t *testing.T) {
	ui := newTestUI()
	_, err := ui.findNode("/anything")
	assert.ErrorIs(t, err, errNotFound)
}

func TestFindNodeRejectsDotDotSegment(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	// Built by concatenation, not filepath.Join, so the literal ".." segment
	// survives into findNode instead of being collapsed away beforehand.
	_, err := ui.findNode(root + "/../etc/passwd")
	assert.ErrorIs(t, err, errOutsideRoot)
}

func TestFindNodeRejectsRelativePath(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	// A relative path can't be made relative to the (absolute) scanned root,
	// so filepath.Rel itself errors.
	_, err := ui.findNode("relative/sub/path")
	assert.ErrorIs(t, err, errOutsideRoot)
}

func TestFindNodeFallsBackToRootPathWhenTopDirPathUnset(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	ui.mu.Lock()
	ui.topDirPath = ""
	ui.mu.Unlock()

	node, err := ui.findNode("")
	require.NoError(t, err)
	assert.Equal(t, root, node.GetPath())
}

func TestTreeEndpointForFileRoot(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	filePath := filepath.Join(root, "big.bin")
	resp, err := http.Get(srv.URL + "/api/v1/tree?path=" + url.QueryEscape(filePath))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tree treePayload
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tree))
	assert.Equal(t, filePath, tree.Path)
	assert.False(t, tree.IsDir)
	assert.Empty(t, tree.Children)
}

func TestTreeEndpointOrdersMultipleDirectoriesBySize(t *testing.T) {
	ui := newTestUI()
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "b")
	require.NoError(t, os.Mkdir(dirA, 0o700))
	require.NoError(t, os.Mkdir(dirB, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dirA, "big.bin"), make([]byte, 8192), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dirB, "small.bin"), make([]byte, 16), 0o600))
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/tree?path=" + url.QueryEscape(root))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tree treePayload
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tree))
	// Two sibling directories queued for expansion exercise the best-first
	// heap's ordering (expansionQueue.Less), not just a single-item queue.
	require.Len(t, tree.Children, 2)
	assert.GreaterOrEqual(t, tree.Children[0].Usage, tree.Children[1].Usage)
}

func TestDeleteNodeEndpointNotFound(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	missing := filepath.Join(root, "does-not-exist")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(missing), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteNodeEndpointRejectsArchiveDescendant(t *testing.T) {
	ui := newTestUI()
	ui.SetArchiveBrowsing(true)
	root := t.TempDir()
	zipPath := filepath.Join(root, "archive.zip")
	makeZip(t, zipPath)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	entryPath := filepath.Join(zipPath, "inner.txt")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(entryPath), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRevealEndpointInvalidBody(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reveal", strings.NewReader("not json"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRevealEndpointNotFound(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	body := strings.NewReader(fmt.Sprintf(`{"path":%q}`, filepath.Join(root, "does-not-exist")))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reveal", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRevealEndpointNoDirectoryToReveal(t *testing.T) {
	ui := newTestUI()
	// A root that is itself a file (not a directory) has no parent, so
	// revealing it hits the "no directory to reveal" guard.
	ui.mu.Lock()
	ui.topDir = &analyze.File{Name: "onlyfile"}
	ui.topDirPath = "onlyfile"
	ui.mu.Unlock()

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reveal", strings.NewReader(`{"path":"onlyfile"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRevealEndpointPropagatesRevealError(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)
	ui.revealPath = func(string) error { return errors.New("boom") }

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	body := strings.NewReader(fmt.Sprintf(`{"path":%q}`, filepath.Join(root, "big.bin")))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reveal", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestWriteNodeErrorDefaultCase(t *testing.T) {
	w := httptest.NewRecorder()
	writeNodeError(w, errors.New("some other failure"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// noFlushRecorder is an http.ResponseWriter that deliberately does not
// implement http.Flusher, unlike httptest.ResponseRecorder.
type noFlushRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newNoFlushRecorder() *noFlushRecorder             { return &noFlushRecorder{header: make(http.Header)} }
func (w *noFlushRecorder) Header() http.Header         { return w.header }
func (w *noFlushRecorder) Write(b []byte) (int, error) { return w.body.Write(b) }
func (w *noFlushRecorder) WriteHeader(status int)      { w.status = status }

func TestHandleEventsFlusherUnsupported(t *testing.T) {
	ui := newTestUI()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	w := newNoFlushRecorder()
	ui.handleEvents(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.status)
}

func TestStatusJSONForClientHidesDeleteForRemote(t *testing.T) {
	msg := `{"state":"done","deleteAllowed":true}`

	out := statusJSONForClient(msg, false)
	var resp statusResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.False(t, resp.DeleteAllowed)

	assert.Equal(t, msg, statusJSONForClient(msg, true), "local clients see the message unchanged")
	assert.Equal(t, "not json", statusJSONForClient("not json", false), "unparseable payloads pass through unchanged")
}

// errWriter is an http.ResponseWriter whose Write always fails, used to
// exercise writeJSON's encode-error logging path.
type errWriter struct{ header http.Header }

func newErrWriter() *errWriter                 { return &errWriter{header: make(http.Header)} }
func (w *errWriter) Header() http.Header       { return w.header }
func (w *errWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func (w *errWriter) WriteHeader(int)           {}

func TestWriteJSONEncodeError(t *testing.T) {
	assert.NotPanics(t, func() {
		writeJSON(newErrWriter(), http.StatusOK, map[string]string{"a": "b"})
	})
}

func TestIsLoopbackHost(t *testing.T) {
	cases := []struct {
		name     string
		host     string
		wantLoop bool
	}{
		{"loopback with port", "127.0.0.1:8080", true},
		{"loopback v6 with port", "[::1]:8080", true},
		{"remote with port", "example.com:80", false},
		{"bare localhost name", "localhost", true},
		{"bare non-loopback host", "example.com", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.wantLoop, isLoopbackHost(c.host))
		})
	}
}

func TestStartUILoopBindError(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer blocker.Close()

	ui := newTestUI()
	ui.listenAddr = blocker.Addr().String()
	err = ui.StartUILoop()
	assert.Error(t, err, "binding an already-used address must fail")
}

func TestStaticHandlerServesIndex(t *testing.T) {
	srv := httptest.NewServer(staticHandler())
	defer srv.Close()

	// Root path serves the SPA entry point.
	resp, err := http.Get(srv.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "no-referrer", resp.Header.Get("Referrer-Policy"),
		"the action token lives in this page's URL; it must never leak via Referer")

	// An unknown non-asset path falls back to index.html (client-side routing).
	resp2, err := http.Get(srv.URL + "/some/spa/route")
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	body, _ := io.ReadAll(resp2.Body)
	lower := strings.ToLower(string(body))
	assert.Truef(t, strings.Contains(lower, "<!doctype html") || strings.Contains(lower, "<html"),
		"fallback body does not look like the SPA index: %q", string(body[:min(80, len(body))]))
}
