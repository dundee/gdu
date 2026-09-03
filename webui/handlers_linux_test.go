//go:build linux

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
)

func TestDeleteNodeEndpointPropagatesRemovalError(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	sub := filepath.Join(root, "sub")
	require.NoError(t, os.Chmod(sub, 0))
	defer func() {
		_ = os.Chmod(sub, 0o700)
	}()

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	path := filepath.Join(sub, "nested.dat")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
	require.NoError(t, err)
	req.Header.Set("X-GDU-Action", "1")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
