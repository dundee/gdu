package webui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestSetCollapsePath(t *testing.T) {
	ui := newTestUI()
	if ui.collapsePath {
		t.Fatal("collapsePath should default to false")
	}
	ui.SetCollapsePath(true)
	if !ui.collapsePath {
		t.Error("SetCollapsePath(true) did not set the field")
	}
}

func TestCreateUIDefaultListenAddr(t *testing.T) {
	ui := CreateUI(io.Discard, "", false, "", true, true, true, true)
	if ui.listenAddr != "localhost:0" {
		t.Errorf("listenAddr = %q, want localhost:0", ui.listenAddr)
	}
	if !ui.UseColors || !ui.ShowApparentSize || !ui.ShowRelativeSize || !ui.UseSIPrefix {
		t.Error("display options not propagated to embedded common.UI")
	}
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

	if err := ui.ListDevices(getter); err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/devices")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var out []deviceJSON
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("devices = %d, want 1", len(out))
	}
	d := out[0]
	if d.Name != "/dev/sda1" || d.MountPoint != "/" || d.Fstype != "ext4" ||
		d.Size != 1000 || d.Free != 400 {
		t.Errorf("unexpected device payload: %+v", d)
	}
}

func TestHandleDevicesEmpty(t *testing.T) {
	ui := newTestUI()

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/devices")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var out []deviceJSON
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("devices = %d, want 0", len(out))
	}
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

	if err := ui.ReadAnalysis(strings.NewReader(sampleReport)); err != nil {
		t.Fatalf("ReadAnalysis: %v", err)
	}

	status := ui.buildStatus()
	if status.State != "done" {
		t.Errorf("state = %q, want done", status.State)
	}
	if status.RootPath != "/home/xxx" {
		t.Errorf("rootPath = %q, want /home/xxx", status.RootPath)
	}

	node, err := ui.findNode("/home/xxx")
	if err != nil {
		t.Fatalf("findNode: %v", err)
	}
	if node.GetName() != "xxx" {
		t.Errorf("root name = %q, want xxx", node.GetName())
	}
	if !node.IsDir() {
		t.Error("root should be a directory")
	}
}

func TestReadAnalysisError(t *testing.T) {
	ui := newTestUI()

	err := ui.ReadAnalysis(strings.NewReader("not valid json"))
	if err == nil {
		t.Fatal("expected error for invalid input")
	}

	status := ui.buildStatus()
	if status.State != "error" {
		t.Errorf("state = %q, want error", status.State)
	}
	if status.Error == "" {
		t.Error("expected non-empty error message in status")
	}
	if ui.scanning {
		t.Error("scanning flag should be cleared after error")
	}
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
	if err := ui.ReadFromStorage(storagePath, "test_dir"); err != nil {
		t.Fatalf("ReadFromStorage: %v", err)
	}

	status := ui.buildStatus()
	if status.State != "done" {
		t.Errorf("state = %q, want done", status.State)
	}

	node, err := ui.findNode("test_dir")
	if err != nil {
		t.Fatalf("findNode: %v", err)
	}
	if node.GetName() != "test_dir" {
		t.Errorf("root name = %q, want test_dir", node.GetName())
	}
	if node.GetItemCount() != 5 {
		t.Errorf("itemCount = %d, want 5", node.GetItemCount())
	}
}

func TestReadFromStorageError(t *testing.T) {
	ui := newTestUI()
	// A non-existent path yields a badger read error.
	err := ui.ReadFromStorage(t.TempDir()+"/badger", "missing_dir")
	if err == nil {
		t.Fatal("expected error reading a missing dir from storage")
	}
}

func TestBuildStatusStates(t *testing.T) {
	// Fresh UI: no topDir yet, not scanning -> reported as scanning.
	ui := newTestUI()
	if got := ui.buildStatus().State; got != "scanning" {
		t.Errorf("initial state = %q, want scanning", got)
	}

	// Explicit scanning flag.
	ui.mu.Lock()
	ui.scanning = true
	ui.mu.Unlock()
	if got := ui.buildStatus().State; got != "scanning" {
		t.Errorf("scanning state = %q, want scanning", got)
	}
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
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		t.Fatalf("statusJSON produced invalid JSON: %v", err)
	}
	if status.RootPath != "/tmp/root" {
		t.Errorf("rootPath = %q, want /tmp/root", status.RootPath)
	}
	if status.Progress.ItemCount != 3 || status.Progress.TotalUsage != 100 ||
		status.Progress.CurrentItem != "current" {
		t.Errorf("unexpected progress: %+v", status.Progress)
	}
}

func TestHandleEventsSSE(t *testing.T) {
	ui := newTestUI()
	ui.mu.Lock()
	ui.topDirPath = "/tmp/root"
	ui.mu.Unlock()

	srv := httptest.NewServer(ui.routes())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type = %q, want text/event-stream", ct)
	}

	// The initial state is pushed immediately on connect.
	reader := bufio.NewReader(resp.Body)
	line, err := readSSEData(t, reader)
	if err != nil {
		t.Fatalf("reading initial SSE frame: %v", err)
	}
	var status statusResponse
	if err := json.Unmarshal([]byte(line), &status); err != nil {
		t.Fatalf("initial SSE frame not valid JSON (%q): %v", line, err)
	}
	if status.RootPath != "/tmp/root" {
		t.Errorf("initial SSE rootPath = %q, want /tmp/root", status.RootPath)
	}

	// A subsequent publish is delivered to the connected subscriber.
	ui.hub.publish(`{"state":"custom"}`)
	line, err = readSSEData(t, reader)
	if err != nil {
		t.Fatalf("reading published SSE frame: %v", err)
	}
	if line != `{"state":"custom"}` {
		t.Errorf("published frame = %q", line)
	}
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
	if last != "" {
		t.Errorf("last = %q, want empty before any publish", last)
	}

	h.publish("hello")
	select {
	case msg := <-ch:
		if msg != "hello" {
			t.Errorf("received %q, want hello", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive published message")
	}

	// A late subscriber gets the most recent message via `last`.
	ch2, last2 := h.subscribe()
	if last2 != "hello" {
		t.Errorf("late subscriber last = %q, want hello", last2)
	}

	h.unsubscribe(ch)
	// unsubscribe closes the channel.
	if _, open := <-ch; open {
		t.Error("channel should be closed after unsubscribe")
	}

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
			if warned != c.wantWarn {
				t.Errorf("warned = %v, want %v (output: %q)", warned, c.wantWarn, buf.String())
			}
		})
	}
}

func TestToNodeJSONFlag(t *testing.T) {
	parent := &analyze.Dir{File: &analyze.File{Name: "root"}}

	// A file with a meaningful flag exposes it in the JSON payload.
	flagged := &analyze.File{Name: "denied", Flag: '!', Parent: parent}
	if got := toNodeJSON(flagged).Flag; got != "!" {
		t.Errorf("flag = %q, want !", got)
	}

	// A blank flag (space) is omitted.
	blank := &analyze.File{Name: "ok", Flag: ' ', Parent: parent}
	if got := toNodeJSON(blank).Flag; got != "" {
		t.Errorf("space flag = %q, want empty", got)
	}

	// A zero flag is also omitted.
	zero := &analyze.File{Name: "zero", Parent: parent}
	if got := toNodeJSON(zero).Flag; got != "" {
		t.Errorf("zero flag = %q, want empty", got)
	}
}

func TestAnalyzePathWithParent(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)

	// First scan the root so we have a parent dir in the tree.
	scan(t, ui, root)

	parent, err := ui.findNode(root)
	if err != nil {
		t.Fatalf("findNode(root): %v", err)
	}

	// Re-scan the "sub" directory into the existing parent. This exercises the
	// parentDir != nil branch: SetParent / RemoveFileByName / AddFile. The
	// re-scanned dir replaces the old child in the parent and keeps the same
	// top-level tree root.
	sub := filepath.Join(root, "sub")
	if err := ui.AnalyzePath(sub, parent); err != nil {
		t.Fatalf("AnalyzePath(sub, parent): %v", err)
	}
	waitDone(t, ui)

	// The refreshed "sub" child is present in the parent and points back to it.
	child, found := childByName(parent, "sub")
	if !found {
		t.Fatal("re-scanned sub not found under parent")
	}
	if child.GetName() != "sub" {
		t.Errorf("child name = %q, want sub", child.GetName())
	}
	if child.GetParent() != parent {
		t.Error("re-scanned sub should have its parent set to the original root")
	}
	if child.GetItemCount() < 1 {
		t.Error("re-scanned sub should have counted its files")
	}
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

func TestStaticHandlerServesIndex(t *testing.T) {
	srv := httptest.NewServer(staticHandler())
	defer srv.Close()

	// Root path serves the SPA entry point.
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("root status = %d, want 200", resp.StatusCode)
	}

	// An unknown non-asset path falls back to index.html (client-side routing).
	resp2, err := http.Get(srv.URL + "/some/spa/route")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("fallback status = %d, want 200", resp2.StatusCode)
	}
	body, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(strings.ToLower(string(body)), "<!doctype html") &&
		!strings.Contains(strings.ToLower(string(body)), "<html") {
		t.Errorf("fallback body does not look like the SPA index: %q", string(body[:min(80, len(body))]))
	}
}
