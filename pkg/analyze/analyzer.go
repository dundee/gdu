package analyze

import (
	"os"
	"sync/atomic"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// BaseAnalyzer provides common logic for all analyzers
type BaseAnalyzer struct {
	progressOutChan         chan common.CurrentProgress
	progressDoneChan        chan struct{}
	progressItemCount       atomic.Int64
	progressTotalUsage      atomic.Int64
	progressCurrentItemName atomic.Value
	currentDir              atomic.Value
	doneChan                common.SignalGroup
	cancelled               atomic.Bool
	wait                    *WaitGroup
	ignoreDir               common.ShouldDirBeIgnored
	ignoreFileType          common.ShouldFileBeIgnored
	followSymlinks          bool
	gitAnnexedSize          bool
	exclusiveSize           bool
	matchesTimeFilterFn     common.TimeFilter
	archiveBrowsing         bool
	progressTicker          *time.Ticker
}

// Init initializes the BaseAnalyzer
func (a *BaseAnalyzer) Init() {
	a.progressOutChan = make(chan common.CurrentProgress, 1)
	a.progressDoneChan = make(chan struct{})
	a.doneChan = make(common.SignalGroup)
	a.wait = (&WaitGroup{}).Init()
	a.progressItemCount.Store(0)
	a.progressTotalUsage.Store(0)
	a.progressCurrentItemName.Store("")
	a.currentDir.Store((*Dir)(nil))
	a.cancelled.Store(false)
	a.progressTicker = time.NewTicker(50 * time.Millisecond)
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

// SetFollowSymlinks sets whether symlink to files should be followed
func (a *BaseAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

// SetShowAnnexedSize sets whether to use annexed size of git-annex files
func (a *BaseAnalyzer) SetShowAnnexedSize(v bool) {
	a.gitAnnexedSize = v
}

// SetExclusiveSize sets whether disk usage of files is the number of bytes not
// shared with other files (Linux only, measured with FIEMAP)
func (a *BaseAnalyzer) SetExclusiveSize(v bool) {
	a.exclusiveSize = v
}

// usageOf returns the disk usage to record for a file. It is the given
// st_blocks based usage unless exclusive size is enabled and could be measured.
func (a *BaseAnalyzer) usageOf(path string, info os.FileInfo, usage int64) int64 {
	if !a.exclusiveSize {
		return usage
	}
	usage, _ = exclusiveUsage(path, info, usage)
	return usage
}

// setFileUsage refines file.Usage like usageOf and, when exclusive size is
// enabled and could be measured, records the bytes in shared extents too.
func (a *BaseAnalyzer) setFileUsage(file *File, path string, info os.FileInfo) {
	if !a.exclusiveSize {
		return
	}
	file.Usage, file.SharedUsage, _ = exclusiveAndSharedUsage(path, info, file.Usage)
}

// SetTimeFilter sets the time filter function for file inclusion
func (a *BaseAnalyzer) SetTimeFilter(matchesTimeFilterFn common.TimeFilter) {
	a.matchesTimeFilterFn = matchesTimeFilterFn
}

// SetArchiveBrowsing sets whether browsing of zip/jar/tar archives is enabled
func (a *BaseAnalyzer) SetArchiveBrowsing(v bool) {
	a.archiveBrowsing = v
}

// SetFileTypeFilter sets the file type filter function
func (a *BaseAnalyzer) SetFileTypeFilter(filter common.ShouldFileBeIgnored) {
	a.ignoreFileType = filter
}

// isFilteringFiles reports whether a file-level filter is active. It mirrors
// common.UI.IsFilteringFiles, which is what decides between UpdateStats and
// UpdateStatsWithFileFiltering, so analyzers that finalize stats during the
// scan can apply the same empty-directory rule (see resolveDirStats).
//
// This relies on the convention that a nil filter means "no filtering":
// common.UI.CreateFileTypeFilter returns nil rather than a predicate that
// accepts everything, and SetTimeFilter is only called when a time filter was
// actually requested. Callers must not pass an always-false filter to mean
// "unfiltered", or a filtered scan's accounting will be applied to it.
func (a *BaseAnalyzer) isFilteringFiles() bool {
	return a.ignoreFileType != nil || a.matchesTimeFilterFn != nil
}

// GetDone returns channel for checking when analysis is done
func (a *BaseAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

// Cancel stops scheduling new scan work. In-flight filesystem operations are
// allowed to finish so the caller can safely use the partial directory tree.
func (a *BaseAnalyzer) Cancel() {
	a.cancelled.Store(true)
}

// IsCancelled reports whether the current scan has been cancelled.
func (a *BaseAnalyzer) IsCancelled() bool {
	return a.cancelled.Load()
}

func (a *BaseAnalyzer) shouldSkipDir(name, path string) bool {
	return a.ignoreDir(name, path) || a.IsCancelled()
}

// ResetProgress prepares the analyzer for a new scan. Call it only after the
// previous scan has completed and before exposing the new scan to cancellation.
func (a *BaseAnalyzer) ResetProgress() {
	if a.progressTicker != nil {
		a.progressTicker.Stop()
	}
	a.Init()
}

func (a *BaseAnalyzer) GetProgress() common.CurrentProgress {
	return common.CurrentProgress{
		CurrentItemName: a.progressCurrentItemName.Load().(string),
		ItemCount:       a.progressItemCount.Load(),
		TotalUsage:      a.progressTotalUsage.Load(),
	}
}

// UpdateProgress updates progress
func (a *BaseAnalyzer) UpdateProgress() {
	ticker := a.progressTicker
	defer ticker.Stop()
	for {
		select {
		case <-a.progressDoneChan:
			return
		case <-ticker.C:
			select {
			case a.progressOutChan <- a.GetProgress():
			default:
			}
		}
	}
}
