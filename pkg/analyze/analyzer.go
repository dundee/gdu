package analyze

import (
	"context"
	"os"
	"sync/atomic"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// BaseAnalyzer provides what all analyzers have in common: the progress of
// the running scan, and the tree it has built so far.
type BaseAnalyzer struct {
	progressItemCount       atomic.Int64
	progressTotalUsage      atomic.Int64
	progressCurrentItemName atomic.Value
	currentDir              atomic.Value
}

// Init initializes the BaseAnalyzer
func (a *BaseAnalyzer) Init() {
	a.progressCurrentItemName.Store("")
	a.currentDir.Store((*Dir)(nil))
}

// scan is one run of an analyzer: its options, and what it keeps while it
// runs.
type scan struct {
	// ctx stops the scan when cancelled
	ctx  context.Context
	opts common.ScanOptions
	// extents is nil unless exclusive size is on
	extents *extentMeter
	// links counts hard-linked files once, for the analyzers that total as
	// they scan
	links linkTracker
}

// begin starts a scan with opts, which cancelling ctx stops, and resets the
// progress.
func (a *BaseAnalyzer) begin(ctx context.Context, opts common.ScanOptions) *scan {
	a.progressItemCount.Store(0)
	a.progressTotalUsage.Store(0)
	a.progressCurrentItemName.Store("")
	a.currentDir.Store((*Dir)(nil))

	s := &scan{ctx: ctx, opts: opts}
	if s.opts.IgnoreDir == nil {
		s.opts.IgnoreDir = func(string, string) bool { return false }
	}
	if opts.ExclusiveSize {
		s.extents = &extentMeter{}
	}
	return s
}

// isCancelled reports whether the scan has been cancelled. The walk then
// stops starting work; what it found so far is kept.
func (s *scan) isCancelled() bool {
	return s.ctx.Err() != nil
}

func (s *scan) shouldSkipDir(name, path string) bool {
	return s.opts.IgnoreDir(name, path) || s.isCancelled()
}

// filteringFiles reports whether the scan leaves out files, which analyzers
// that total as they scan need for the empty-directory rule (see
// resolveDirStats). A nil filter means no filtering.
func (s *scan) filteringFiles() bool {
	return s.opts.FilteringFiles()
}

// measureSharedUsage returns the disk usage to record for a file and the
// bytes it shares with other files. With exclusive size off, or when the file
// cannot be measured, that is the given st_blocks based usage with nothing
// shared.
func (s *scan) measureSharedUsage(path string, info os.FileInfo, usage int64) (exclusive, shared int64) {
	if s.extents == nil {
		return usage, 0
	}
	return s.extents.measure(path, info, usage)
}

// MeasureSharedUsage returns the bytes of the file at path not shared with
// other files, and the bytes it shares (Linux and macOS only, measured with
// FIEMAP and getattrlist respectively). usage is its st_blocks based usage,
// which is returned, with nothing shared, when the file cannot be measured.
func MeasureSharedUsage(path string, info os.FileInfo, usage int64) (exclusive, shared int64) {
	return (&extentMeter{}).measure(path, info, usage)
}

// setCurrentDir stores the root directory currently being analyzed so it can be
// inspected (e.g. previewed) while the scan is still running.
func (a *BaseAnalyzer) setCurrentDir(dir *Dir) {
	a.currentDir.Store(dir)
}

// GetCurrentDir returns an independent snapshot of the directory tree built so
// far, or nil if no analysis has started yet. Callers may aggregate or navigate
// the snapshot without mutating the live scan state.
func (a *BaseAnalyzer) GetCurrentDir() fs.Item {
	dir, _ := a.currentDir.Load().(*Dir)
	if dir == nil {
		return nil
	}
	return snapshotDir(dir, nil)
}

// GetProgress returns the progress of the running scan, or of the last one.
func (a *BaseAnalyzer) GetProgress() common.CurrentProgress {
	return common.CurrentProgress{
		CurrentItemName: a.progressCurrentItemName.Load().(string),
		ItemCount:       a.progressItemCount.Load(),
		TotalUsage:      a.progressTotalUsage.Load(),
	}
}
