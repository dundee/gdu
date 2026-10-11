package webui

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// errSnapshotReadOnly is returned for mutating requests aimed at a snapshot.
var errSnapshotReadOnly = errors.New("snapshots are read-only")

// errSnapshotUnsupported is returned when the tree is not held in memory.
var errSnapshotUnsupported = errors.New("scanning snapshots is not supported with --db")

// openSnapshot scans a skipped btrfs subvolume as its own scope the first time it is opened
// and returns the scanned directory that replaced the placeholder. Any other item is returned
// as is. The scan runs on the request, serialized with other actions by actionMu, and is
// cancelled when the client goes away.
func (ui *UI) openSnapshot(ctx context.Context, node fs.Item) (fs.Item, error) {
	if !analyze.IsUnscannedSnapshot(node) {
		return node, nil
	}

	ui.actionMu.Lock()
	defer ui.actionMu.Unlock()

	ui.mu.RLock()
	scanning := ui.scanning
	ui.mu.RUnlock()
	if scanning {
		return node, nil // tree is still being built, show the placeholder
	}

	// another request may have scanned it while we waited for the lock
	current, err := ui.findNode(node.GetPath())
	if err != nil {
		return nil, err
	}
	if !analyze.IsUnscannedSnapshot(current) {
		return current, nil
	}

	analyzer := analyze.CreateSubvolumeAnalyzer()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			analyzer.Cancel()
		case <-finished:
		}
	}()

	log.Printf("webui: scanning snapshot %s", current.GetPath())
	scanned := analyzer.AnalyzeDir(current.GetPath(), ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter())
	if ctx.Err() != nil {
		log.Printf("webui: scan of snapshot %s cancelled", current.GetPath())
		return nil, ctx.Err()
	}

	if err := analyze.AttachSnapshotScope(current, scanned); err != nil {
		log.Printf("webui: attaching snapshot %s: %s", current.GetPath(), err)
		return nil, fmt.Errorf("%w: %w", errSnapshotUnsupported, err)
	}
	return scanned, nil
}
