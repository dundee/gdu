// Package webui implements a web-based frontend for Gdu. It serves an embedded
// React single-page application and a small, bounded JSON API over the
// in-memory analysis tree, streaming scan progress via Server-Sent Events.
package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/dundee/gdu/v6/internal/cleanup"
	"github.com/dundee/gdu/v6/internal/common"
	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/remove"
	"github.com/dundee/gdu/v6/internal/session"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/device"
	"github.com/dundee/gdu/v6/pkg/fs"
)

// UI is the web frontend. It embeds the shared *common.UI so that all the
// generic Set* configuration methods are provided for free; only the
// frontend-specific methods are implemented in this package.
type UI struct {
	*common.UI

	output      io.Writer
	listenAddr  string
	openBrowser bool
	browserCmd  string
	revealPath  func(string) error
	actionToken string

	getter  device.DevicesInfoGetter
	devices device.Devices

	session *session.Session

	mu         sync.RWMutex
	actionMu   sync.Mutex
	topDirPath string
	scanning   bool
	scanErr    error
	progress   analyze.Progress
	noDelete   bool
	trasher    remove.Trash

	classifier *cleanup.Classifier
	cleanup    cleanupCache
	// treeGen counts changes to the tree that keep its root, such as a
	// deletion or a rescan of a subdirectory.
	treeGen atomic.Uint64
	// removalSeq is odd while a deletion runs, from before the tree changes
	// until the classification has been updated for it.
	removalSeq atomic.Uint64
	// classifying is set while the classification runs in the background.
	classifying atomic.Bool

	hub *hub
}

// readTree holds the lock of the session's tree for reading until the
// returned function is called. Whoever holds it must not take it again.
func (ui *UI) readTree() func() {
	lock := ui.session.TreeLock()
	lock.RLock()
	return lock.RUnlock
}

// scannedRoots returns the paths of the scanned roots, the only places
// deletion may change anything. It takes the tree's read lock.
func (ui *UI) scannedRoots() []string {
	return ui.session.RootPaths()
}

// CreateUI returns the web UI for the settings s, scanning with analyzer. It
// fails when the cleanup rules of the settings are invalid.
func CreateUI(output io.Writer, analyzer analyze.Analyzer, s *config.Settings) (*UI, error) {
	classifier, err := cleanup.NewClassifier(s.Web.Cleanup, userHomeDir())
	if err != nil {
		return nil, fmt.Errorf("cleanup config: %w", err)
	}
	ui := &UI{
		UI:          common.NewUI(analyzer, s),
		output:      output,
		listenAddr:  s.Web.Listen,
		openBrowser: s.Web.OpenBrowser,
		browserCmd:  s.Web.Browser,
		noDelete:    s.Delete.Disabled,
		revealPath:  revealInFileManager,
		trasher:     remove.MoveItemToTrash,
		classifier:  classifier,
		actionToken: generateActionToken(),
		hub:         newHub(),
	}
	if s.Delete.TrashCommand != "" {
		ui.trasher = remove.TrashCommand(s.Delete.TrashCommand)
	}
	ui.session = session.New(ui.UI)
	if ui.listenAddr == "" {
		ui.listenAddr = "localhost:0"
	}
	if !s.Display.SIPrefix {
		ui.SetBlockSizeFromEnvironment()
	}
	return ui, nil
}

// ListDevices loads mounted devices so they can be served to the browser.
func (ui *UI) ListDevices(getter device.DevicesInfoGetter) error {
	ui.getter = getter
	devices, err := getter.GetDevicesInfo()
	if err != nil {
		return err
	}
	ui.mu.Lock()
	ui.devices = devices
	ui.mu.Unlock()
	return nil
}

// AnalyzePath analyzes disk usage in the given path in the background while the
// HTTP server serves progress and partial results.
func (ui *UI) AnalyzePath(path string, _ fs.Item) error {
	ui.scan([]string{path}, path)
	return nil
}

// AnalyzePaths scans several roots and serves them under a virtual top level
// dir.
func (ui *UI) AnalyzePaths(paths []string) error {
	if len(paths) == 1 {
		return ui.AnalyzePath(paths[0], nil)
	}
	ui.scan(paths, analyze.VirtualRootName)
	return nil
}

// scan scans paths in the background, showing shownPath as the root path
// until the scan is done.
func (ui *UI) scan(paths []string, shownPath string) {
	ui.mu.Lock()
	ui.scanning = true
	ui.scanErr = nil
	ui.topDirPath = shownPath
	ui.mu.Unlock()

	go func() {
		root := ui.session.Scan(context.Background(), paths, ui.observeProgress)

		ui.mu.Lock()
		ui.topDirPath = root.GetPath()
		ui.scanning = false
		ui.mu.Unlock()
		ui.treeChanged()

		ui.hub.publish(ui.statusJSON())
	}()
}

// observeProgress keeps and broadcasts the progress of a running scan.
func (ui *UI) observeProgress(p session.Progress) {
	ui.mu.Lock()
	ui.progress = p.Progress
	ui.mu.Unlock()
	ui.hub.publish(ui.statusJSON())
}

// ReadAnalysis reads an analysis report from a JSON reader and serves it.
func (ui *UI) ReadAnalysis(input io.Reader) error {
	ui.mu.Lock()
	ui.scanning = true
	ui.mu.Unlock()

	dir, err := ui.session.Load(input)
	if err != nil {
		ui.mu.Lock()
		ui.scanning = false
		ui.scanErr = err
		ui.mu.Unlock()
		return err
	}
	ui.treeLoaded(dir)
	return nil
}

// ReadFromStorage serves the analysis of path stored by an earlier scan with
// --db.
func (ui *UI) ReadFromStorage(path string) error {
	dir, err := ui.session.Open(path)
	if err != nil {
		return err
	}
	ui.treeLoaded(dir)
	return nil
}

// treeLoaded serves dir, the tree just loaded into the session.
func (ui *UI) treeLoaded(dir fs.Item) {
	ui.mu.Lock()
	ui.topDirPath = dir.GetPath()
	ui.scanning = false
	ui.mu.Unlock()
	ui.treeChanged()
}

// statusJSON renders the current scan status as a JSON string for SSE.
func (ui *UI) statusJSON() string {
	data, err := json.Marshal(ui.buildStatus())
	if err != nil {
		slog.Warn("webui: marshaling status", "err", err)
		return `{"state":"error"}`
	}
	return string(data)
}
