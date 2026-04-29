package analyze

import (
	"sync/atomic"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
)

// BaseAnalyzer provides common logic for all analyzers
type BaseAnalyzer struct {
	progressOutChan         chan common.CurrentProgress
	progressDoneChan        chan struct{}
	progressItemCount       atomic.Int64
	progressTotalUsage      atomic.Int64
	progressCurrentItemName atomic.Value
	doneChan                common.SignalGroup
	wait                    *WaitGroup
	ignoreDir               common.ShouldDirBeIgnored
	ignoreFileType          common.ShouldFileBeIgnored
	followSymlinks          bool
	gitAnnexedSize          bool
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
	a.progressTicker = time.NewTicker(50 * time.Millisecond)
}

// SetFollowSymlinks sets whether symlink to files should be followed
func (a *BaseAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

// SetShowAnnexedSize sets whether to use annexed size of git-annex files
func (a *BaseAnalyzer) SetShowAnnexedSize(v bool) {
	a.gitAnnexedSize = v
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

// GetDone returns channel for checking when analysis is done
func (a *BaseAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

// ResetProgress resets the analyzer state
func (a *BaseAnalyzer) ResetProgress() {
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
