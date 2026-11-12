// Package webui implements a web-based frontend for Gdu. It serves an embedded
// React single-page application and a small, bounded JSON API over the
// in-memory analysis tree, streaming scan progress via Server-Sent Events.
package webui

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/session"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/cleanup"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/pkg/remove"
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

	mu           sync.RWMutex
	actionMu     sync.Mutex
	topDirPath   string
	collapsePath bool
	scanning     bool
	scanErr      error
	progress     common.CurrentProgress
	noDelete     bool
	trasher      func(fs.Item, fs.Item) error

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

// SetNoDelete disables destructive actions in the web UI.
func (ui *UI) SetNoDelete() {
	ui.mu.Lock()
	ui.noDelete = true
	ui.mu.Unlock()
}

// scannedRoots returns the paths of the scanned roots, the only places
// deletion may change anything.
func (ui *UI) scannedRoots() []string {
	return ui.session.RootPaths()
}

// SetTrashCommand replaces the built-in trash with an external command, as
// used by --trash-command. See remove.TrashCommand for the command contract.
func (ui *UI) SetTrashCommand(command string) {
	ui.trasher = remove.TrashCommand(command)
}

// CreateUI creates a new web UI.
func CreateUI(
	output io.Writer,
	listenAddr string,
	openBrowser bool,
	browserCmd string,
	useColors bool,
	showApparentSize bool,
	showRelativeSize bool,
	useSIPrefix bool,
) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowApparentSize: showApparentSize,
			ShowRelativeSize: showRelativeSize,
			UseSIPrefix:      useSIPrefix,
			Analyzer:         analyze.CreateAnalyzer(),
		},
		output:      output,
		listenAddr:  listenAddr,
		openBrowser: openBrowser,
		browserCmd:  browserCmd,
		revealPath:  revealInFileManager,
		trasher:     remove.MoveItemToTrash,
		actionToken: generateActionToken(),
		hub:         newHub(),
	}
	ui.session = session.New(ui.UI)
	ui.classifier = cleanup.NewDefaultClassifier(userHomeDir())
	if listenAddr == "" {
		ui.listenAddr = "localhost:0"
	}
	if !useSIPrefix {
		ui.SetBlockSizeFromEnvironment()
	}
	return ui
}

// SetCollapsePath sets whether single-child directory chains are collapsed.
func (ui *UI) SetCollapsePath(value bool) {
	ui.collapsePath = value
}

// SetShowSymlinkTarget is a no-op for the web UI (rendering is browser-side).
func (ui *UI) SetShowSymlinkTarget(value bool) {
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
	ui.progress = p.CurrentProgress
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
func (ui *UI) ReadFromStorage(_, path string) error {
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
		log.Printf("webui: marshaling status: %s", err)
		return `{"state":"error"}`
	}
	return string(data)
}
