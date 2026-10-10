package tui

import (
	"runtime/debug"
	"time"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

const snapshotReadOnlyMessage = " Snapshots are read-only, use btrfs subvolume delete"

// selectionInSnapshotScope reports whether the marked items, or the selected one
// when nothing is marked, are a skipped subvolume or live inside one.
func (ui *UI) selectionInSnapshotScope() bool {
	if len(ui.markedRows) > 0 {
		for row := range ui.markedRows {
			if item, ok := ui.table.GetCell(row, 0).GetReference().(fs.Item); ok && analyze.InSnapshotScope(item) {
				return true
			}
		}
		return false
	}
	row, column := ui.table.GetSelection()
	item, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	return ok && item != ui.currentDir.GetParent() && analyze.InSnapshotScope(item)
}

// scanSnapshotScope scans a skipped subvolume as its own scope, swaps the result
// into the tree in place of the placeholder and shows it.
func (ui *UI) scanSnapshotScope(placeholder fs.Item) {
	if ui.scanning {
		return
	}
	if _, ok := placeholder.GetParent().(*analyze.Dir); !ok {
		log.Printf("Cannot scan snapshot %s: parent is not held in memory", placeholder.GetPath())
		ui.showMessage(" Scanning snapshots is not supported with --db")
		return
	}

	path := placeholder.GetPath()
	log.Printf("Scanning snapshot %s as its own scope", path)

	ui.showScanProgressModal(" Scanning snapshot... ")
	analyzer := analyze.CreateSubvolumeAnalyzer()
	ui.scopeAnalyzer = analyzer
	ui.scanCancelRequested.Store(false)
	go ui.updateProgress(analyzer, analyzer.GetDone())

	ui.scanStart = time.Now()
	ui.scanning = true
	ui.scanCancelled = false
	go func() {
		defer debug.FreeOSMemory()
		scanned := analyzer.AnalyzeDir(path, ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter())
		cancelled := ui.scanCancelRequested.Load()

		var attachErr error
		if !cancelled {
			attachErr = analyze.AttachSnapshotScope(placeholder, scanned)
		}

		ui.app.QueueUpdateDraw(func() {
			ui.scanning = false
			ui.scanCancelled = false
			ui.scopeAnalyzer = nil
			ui.pages.RemovePage("progress")
			switch {
			case cancelled:
				log.Printf("Scan of snapshot %s cancelled, keeping the placeholder", path)
				ui.showMessage(" Snapshot scan cancelled")
			case attachErr != nil:
				log.Printf("Cannot attach scanned snapshot %s: %s", path, attachErr)
				ui.showErr("Error attaching snapshot", attachErr)
			default:
				ui.currentDir = scanned
				ui.hideFilterInput()
				ui.hideTypeFilterInput()
				ui.markedRows = make(map[int]struct{})
				ui.ignoredRows = make(map[int]struct{})
				ui.showDir()
			}
		})

		if ui.done != nil {
			ui.done <- struct{}{}
		}
	}()
}
