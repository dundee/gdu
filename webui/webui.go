// Package webui implements a web-based frontend for Gdu. It serves an embedded
// React single-page application and a small, bounded JSON API over the
// in-memory analysis tree, streaming scan progress via Server-Sent Events.
package webui

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/pkg/remove"
	"github.com/dundee/gdu/v5/report"
)

// progressPollInterval is how often the scan progress is sampled and broadcast.
const progressPollInterval = 100 * time.Millisecond

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

	mu           sync.RWMutex
	actionMu     sync.Mutex
	topDir       fs.Item
	topDirPath   string
	linkedItems  fs.HardLinkedItems
	collapsePath bool
	scanning     bool
	scanErr      error
	progress     common.CurrentProgress
	noDelete     bool
	trasher      func(fs.Item, fs.Item) error

	hub *hub
}

// SetNoDelete disables destructive actions in the web UI.
func (ui *UI) SetNoDelete() {
	ui.mu.Lock()
	ui.noDelete = true
	ui.mu.Unlock()
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
		revealPath:  openPath,
		trasher:     remove.MoveItemToTrash,
		actionToken: generateActionToken(),
		linkedItems: make(fs.HardLinkedItems),
		hub:         newHub(),
	}
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
func (ui *UI) AnalyzePath(path string, parentDir fs.Item) error {
	ui.mu.Lock()
	ui.scanning = true
	ui.scanErr = nil
	ui.topDirPath = path
	ui.mu.Unlock()

	ui.Analyzer.ResetProgress()

	scanDone := make(chan struct{})
	go ui.pollProgress(scanDone)

	go func() {
		dir := ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter())

		if parentDir != nil {
			dir.SetParent(parentDir)
			parentDir.RemoveFileByName(dir.GetName())
			parentDir.AddFile(dir)
		}

		if ui.IsFilteringFiles() {
			dir.UpdateStatsWithFileFiltering(ui.linkedItems)
		} else {
			dir.UpdateStats(ui.linkedItems)
		}

		ui.mu.Lock()
		if parentDir == nil {
			ui.topDir = dir
			ui.topDirPath = dir.GetPath()
		}
		ui.scanning = false
		ui.mu.Unlock()

		close(scanDone)
		ui.hub.publish(ui.statusJSON())
	}()

	return nil
}

// AnalyzePaths scans several roots one after another and serves them under a
// virtual top level dir. The roots are scanned sequentially because the
// analyzer owns a single set of progress channels.
func (ui *UI) AnalyzePaths(paths []string) error {
	if len(paths) == 1 {
		return ui.AnalyzePath(paths[0], nil)
	}

	ui.mu.Lock()
	ui.scanning = true
	ui.scanErr = nil
	ui.topDirPath = analyze.VirtualRootName
	ui.mu.Unlock()

	go func() {
		roots := make([]fs.Item, 0, len(paths))
		for _, path := range paths {
			ui.Analyzer.ResetProgress()

			scanDone := make(chan struct{})
			go ui.pollProgress(scanDone)

			roots = append(
				roots,
				ui.Analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter()),
			)
			close(scanDone)
		}

		dir := analyze.CreateVirtualRootDir(roots...)
		if ui.IsFilteringFiles() {
			dir.UpdateStatsWithFileFiltering(ui.linkedItems)
		} else {
			dir.UpdateStats(ui.linkedItems)
		}

		ui.mu.Lock()
		ui.topDir = dir
		ui.topDirPath = dir.GetPath()
		ui.scanning = false
		ui.mu.Unlock()

		ui.hub.publish(ui.statusJSON())
	}()

	return nil
}

// ReadAnalysis reads an analysis report from a JSON reader and serves it.
func (ui *UI) ReadAnalysis(input io.Reader) error {
	ui.mu.Lock()
	ui.scanning = true
	ui.mu.Unlock()

	dir, err := report.ReadAnalysis(input)
	if err != nil {
		ui.mu.Lock()
		ui.scanning = false
		ui.scanErr = err
		ui.mu.Unlock()
		return err
	}
	dir.UpdateStats(ui.linkedItems)

	ui.mu.Lock()
	ui.topDir = dir
	ui.topDirPath = dir.GetPath()
	ui.scanning = false
	ui.mu.Unlock()
	return nil
}

// ReadFromStorage reads a previously stored analysis and serves it.
func (ui *UI) ReadFromStorage(storagePath, path string) error {
	storage := analyze.NewStorage(storagePath, path)
	closeFn := storage.Open()
	defer closeFn()

	dir, err := storage.GetDirForPath(path)
	if err != nil {
		return err
	}
	dir.UpdateStats(ui.linkedItems)

	ui.mu.Lock()
	ui.topDir = dir
	ui.topDirPath = dir.GetPath()
	ui.scanning = false
	ui.mu.Unlock()
	return nil
}

// pollProgress samples the analyzer progress and broadcasts it until the scan
// goroutine signals completion by closing scanDone.
func (ui *UI) pollProgress(scanDone <-chan struct{}) {
	ticker := time.NewTicker(progressPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-scanDone:
			return
		case <-ticker.C:
			p := ui.Analyzer.GetProgress()
			ui.mu.Lock()
			ui.progress = p
			ui.mu.Unlock()
			ui.hub.publish(ui.statusJSON())
		}
	}
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
