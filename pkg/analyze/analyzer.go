package analyze

import (
	"github.com/dundee/gdu/v5/internal/common"
)

type BaseAnalyzer struct {
	progress            *common.CurrentProgress
	progressChan        chan common.CurrentProgress
	progressOutChan     chan common.CurrentProgress
	progressDoneChan    chan struct{}
	doneChan            common.SignalGroup
	wait                *WaitGroup
	ignoreDir           common.ShouldDirBeIgnored
	ignoreFileType      common.ShouldFileBeIgnored
	followSymlinks      bool
	gitAnnexedSize      bool
	matchesTimeFilterFn common.TimeFilter
	archiveBrowsing     bool
}

func (a *BaseAnalyzer) init() {
	a.progress = &common.CurrentProgress{
		ItemCount: 0,
		TotalSize: int64(0),
	}
	a.progressChan = make(chan common.CurrentProgress, 1)
	a.progressOutChan = make(chan common.CurrentProgress, 1)
	a.progressDoneChan = make(chan struct{})
	a.doneChan = make(common.SignalGroup)
	a.wait = (&WaitGroup{}).Init()
}

func (a *BaseAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

func (a *BaseAnalyzer) SetShowAnnexedSize(v bool) {
	a.gitAnnexedSize = v
}

func (a *BaseAnalyzer) SetTimeFilter(matchesTimeFilterFn common.TimeFilter) {
	a.matchesTimeFilterFn = matchesTimeFilterFn
}

func (a *BaseAnalyzer) SetArchiveBrowsing(v bool) {
	a.archiveBrowsing = v
}

func (a *BaseAnalyzer) SetFileTypeFilter(filter common.ShouldFileBeIgnored) {
	a.ignoreFileType = filter
}

func (a *BaseAnalyzer) GetProgressChan() chan common.CurrentProgress {
	return a.progressOutChan
}

func (a *BaseAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

func (a *BaseAnalyzer) ResetProgress() {
	a.init()
}

func (a *BaseAnalyzer) updateProgress() {
	for {
		select {
		case <-a.progressDoneChan:
			return
		case progress := <-a.progressChan:
			a.progress.CurrentItemName = progress.CurrentItemName
			a.progress.ItemCount += progress.ItemCount
			a.progress.TotalSize += progress.TotalSize
		}

		select {
		case a.progressOutChan <- *a.progress:
		default:
		}
	}
}
