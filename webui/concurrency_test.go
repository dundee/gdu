package webui

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Requests read the tree while deletions change it and the classification
// walks it. Run with -race.
func TestRequestsWhileDeletingAndClassifying(t *testing.T) {
	ui := newTestUI()
	root := t.TempDir()
	for i := range 40 {
		dir := filepath.Join(root, fmt.Sprintf("d%02d", i), "node_modules")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "f"), make([]byte, 100), 0o600))
	}
	scan(t, ui, root)
	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	get := func(endpoint string) {
		resp, err := http.Get(srv.URL + endpoint + "?path=" + url.QueryEscape(root))
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}
	done := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
					get("/api/v1/nodes")
					get("/api/v1/tree")
					get("/api/v1/categories")
				}
			}
		})
	}

	for i := range 40 {
		path := filepath.Join(root, fmt.Sprintf("d%02d", i))
		req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/nodes?path="+url.QueryEscape(path), nil)
		require.NoError(t, err)
		req.Header.Set("X-GDU-Action", ui.actionToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusNoContent, resp.StatusCode, path)
	}
	close(done)
	readers.Wait()

	unlock := ui.readTree()
	defer unlock()
	assert.Equal(t, int64(1), ui.session.Root().GetItemCount())
}
