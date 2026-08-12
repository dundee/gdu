package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

func newTestUI() *UI {
	return CreateUI(io.Discard, "127.0.0.1:0", false, "", false, false, false, false)
}

// makeTree creates a temporary directory tree and returns its root path.
func makeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "big.bin"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "small.txt"), make([]byte, 16), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nested.dat"), make([]byte, 1024), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func scan(t *testing.T, ui *UI, path string) {
	t.Helper()
	if err := ui.AnalyzePath(path, nil); err != nil {
		t.Fatalf("AnalyzePath: %v", err)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var status statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.State != "done" {
		t.Errorf("state = %q, want done", status.State)
	}
	if status.RootPath != root {
		t.Errorf("rootPath = %q, want %q", status.RootPath, root)
	}
}

func TestNodesEndpoint(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape(root))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var node nodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		t.Fatal(err)
	}

	if !node.Node.IsDir {
		t.Error("root node should be a directory")
	}
	if len(node.Breadcrumbs) != 1 {
		t.Errorf("breadcrumbs = %d, want 1", len(node.Breadcrumbs))
	}
	if len(node.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(node.Children))
	}
	// Default sort is by disk usage descending. Assert the returned order is
	// monotonically non-increasing rather than depending on fixture specifics
	// (sparse files can have near-zero disk usage regardless of apparent size).
	for i := 1; i < len(node.Children); i++ {
		if node.Children[i-1].Usage < node.Children[i].Usage {
			t.Errorf("children not sorted by usage desc: %+v", node.Children)
			break
		}
	}
	for _, c := range node.Children {
		if c.Size <= 0 {
			t.Errorf("child %q has non-positive size", c.Name)
		}
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
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var node nodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		t.Fatal(err)
	}
	if node.Node.Name != "sub" {
		t.Errorf("node name = %q, want sub", node.Node.Name)
	}
	if len(node.Breadcrumbs) != 2 {
		t.Errorf("breadcrumbs = %d, want 2 (root, sub)", len(node.Breadcrumbs))
	}
	if len(node.Children) != 1 || node.Children[0].Name != "nested.dat" {
		t.Errorf("unexpected children: %+v", node.Children)
	}
}

func TestNodesPathTraversalRejected(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape("/etc"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestNodesNotFound(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)
	scan(t, ui, root)

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	missing := filepath.Join(root, "does-not-exist")
	resp, err := http.Get(srv.URL + "/api/v1/nodes?path=" + url.QueryEscape(missing))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
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
		if gotBy != c.wantBy || gotOrder != c.wantOrder {
			t.Errorf("parseSort(%q,%q) = (%v,%v), want (%v,%v)",
				c.sort, c.order, gotBy, gotOrder, c.wantBy, c.wantOrder)
		}
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
		if err != nil {
			t.Fatalf("browserCommand error: %v", err)
		}
		if name != c.wantName {
			t.Errorf("name = %q, want %q", name, c.wantName)
		}
		if len(args) != len(c.wantArgs) {
			t.Fatalf("args = %v, want %v", args, c.wantArgs)
		}
		for i := range args {
			if args[i] != c.wantArgs[i] {
				t.Errorf("args[%d] = %q, want %q", i, args[i], c.wantArgs[i])
			}
		}
	}
}
