package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/cleanup"
)

// makeCleanupTree creates a project with build artifacts and a model file:
//
//	root/app/package.json
//	root/app/node_modules/lib.js
//	root/models/a.gguf
//	root/docs/readme.txt
func makeCleanupTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel string, size int) {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, make([]byte, size), 0o600))
	}
	write("app/package.json", 16)
	write("app/node_modules/lib.js", 8192)
	write("models/a.gguf", 16384)
	write("docs/readme.txt", 4096)
	return root
}

func getJSON(t *testing.T, srv *httptest.Server, path string, out any) int {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	if out != nil && resp.StatusCode == http.StatusOK {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
	}
	return resp.StatusCode
}

func categoryAmounts(resp categoriesResponse) map[cleanup.Category]cleanup.Amount {
	out := make(map[cleanup.Category]cleanup.Amount)
	for _, c := range resp.Categories {
		out[c.ID] = c.Amount
	}
	return out
}

func TestCategoriesEndpointScopesToPath(t *testing.T) {
	ui := newTestUI()
	root := makeCleanupTree(t)
	scan(t, ui, root)
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	var all categoriesResponse
	require.Equal(t, http.StatusOK, getJSON(t, srv, "/api/v1/categories", &all))
	amounts := categoryAmounts(all)
	assert.Len(t, all.Categories, len(cleanup.Categories))
	assert.Equal(t, int64(8192), amounts[cleanup.BuildArtifact].Size)
	assert.Equal(t, int64(16384), amounts[cleanup.MLModel].Size)
	assert.Greater(t, all.Total.Size, amounts[cleanup.MLModel].Size+amounts[cleanup.BuildArtifact].Size)

	var models categoriesResponse
	require.Equal(t, http.StatusOK,
		getJSON(t, srv, "/api/v1/categories?path="+url.QueryEscape(filepath.Join(root, "models")), &models))
	amounts = categoryAmounts(models)
	assert.Equal(t, int64(16384), amounts[cleanup.MLModel].Size)
	assert.Zero(t, amounts[cleanup.BuildArtifact].Size)

	assert.Equal(t, http.StatusNotFound,
		getJSON(t, srv, "/api/v1/categories?path="+url.QueryEscape(filepath.Join(root, "missing")), nil))
}

func TestCategoriesEndpointUnavailableWhileScanning(t *testing.T) {
	ui := newTestUI()
	scan(t, ui, makeCleanupTree(t))
	ui.mu.Lock()
	ui.scanning = true
	ui.mu.Unlock()
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	assert.Equal(t, http.StatusServiceUnavailable, getJSON(t, srv, "/api/v1/categories", nil))
	assert.Equal(t, http.StatusServiceUnavailable, getJSON(t, srv, "/api/v1/tree?pivot=category", nil))
}

func TestTreeEndpointAnnotatesCandidates(t *testing.T) {
	ui := newTestUI()
	root := makeCleanupTree(t)
	scan(t, ui, root)
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	var tree treeJSON
	require.Equal(t, http.StatusOK, getJSON(t, srv, "/api/v1/tree", &tree))
	assert.Equal(t, int64(16384), tree.CategoryTotals[cleanup.MLModel].Size)

	byName := func(n *treeJSON, name string) *treeJSON {
		for _, c := range n.Children {
			if c.Name == name {
				return c
			}
		}
		t.Fatalf("child %q missing", name)
		return nil
	}
	modules := byName(byName(&tree, "app"), "node_modules")
	assert.Equal(t, cleanup.BuildArtifact, modules.Category)
	assert.Equal(t, cleanup.Safe, modules.Confidence)
	assert.NotEmpty(t, modules.Hint)

	docs := byName(&tree, "docs")
	assert.Empty(t, docs.Category)
	assert.Empty(t, docs.CategoryTotals)
}

func TestPivotTreeGroupsCandidatesByCategory(t *testing.T) {
	ui := newTestUI()
	root := makeCleanupTree(t)
	scan(t, ui, root)
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	var tree treeJSON
	require.Equal(t, http.StatusOK, getJSON(t, srv, "/api/v1/tree?pivot=category", &tree))
	assert.Equal(t, root, tree.Path)
	require.Len(t, tree.Children, 2)

	// categories come in display order
	build, models := tree.Children[0], tree.Children[1]
	assert.Equal(t, pivotPathPrefix+string(cleanup.BuildArtifact), build.Path)
	assert.Equal(t, int64(8192), build.Size)
	require.Len(t, build.Children, 1)
	assert.Equal(t, filepath.Join(root, "app", "node_modules"), build.Children[0].Path)

	assert.Equal(t, cleanup.MLModel, models.Category)
	require.Len(t, models.Children, 1)
	assert.Equal(t, filepath.Join(root, "models", "a.gguf"), models.Children[0].Path)

	// scoped to a subdirectory, only its candidates remain
	var scoped treeJSON
	require.Equal(t, http.StatusOK,
		getJSON(t, srv, "/api/v1/tree?pivot=category&path="+url.QueryEscape(filepath.Join(root, "app")), &scoped))
	require.Len(t, scoped.Children, 1)
	assert.Equal(t, cleanup.BuildArtifact, scoped.Children[0].Category)
}

func TestPivotCategoryNodesAreNotActionable(t *testing.T) {
	ui := newTestUI()
	scan(t, ui, makeCleanupTree(t))
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete,
		srv.URL+"/api/v1/nodes?path="+url.QueryEscape(pivotPathPrefix+string(cleanup.MLModel)), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestClassificationUpdatedAfterDelete(t *testing.T) {
	ui := newTestUI()
	root := makeCleanupTree(t)
	scan(t, ui, root)
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	var before categoriesResponse
	require.Equal(t, http.StatusOK, getJSON(t, srv, "/api/v1/categories", &before))
	require.NotZero(t, categoryAmounts(before)[cleanup.MLModel].Size)
	cached := ui.cleanupResult()

	req, err := http.NewRequest(http.MethodDelete,
		srv.URL+"/api/v1/nodes?path="+url.QueryEscape(filepath.Join(root, "models", "a.gguf")), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", ui.actionToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	var after categoriesResponse
	require.Equal(t, http.StatusOK, getJSON(t, srv, "/api/v1/categories", &after))
	assert.Zero(t, categoryAmounts(after)[cleanup.MLModel].Size)
	assert.NotZero(t, categoryAmounts(after)[cleanup.BuildArtifact].Size)
	assert.Same(t, cached, ui.cleanupResult(), "a deletion updates the classification instead of recomputing it")
}

func TestSetCleanupConfigAppliesExclusions(t *testing.T) {
	ui := newTestUI()
	root := makeCleanupTree(t)
	require.NoError(t, ui.SetCleanupConfig(cleanup.Config{Exclude: []string{filepath.Join(root, "app")}}))
	scan(t, ui, root)
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	var resp categoriesResponse
	require.Equal(t, http.StatusOK, getJSON(t, srv, "/api/v1/categories", &resp))
	assert.Zero(t, categoryAmounts(resp)[cleanup.BuildArtifact].Size)

	assert.Error(t, ui.SetCleanupConfig(cleanup.Config{Rules: []cleanup.Rule{{Category: "junk", Dir: "x"}}}))
}
