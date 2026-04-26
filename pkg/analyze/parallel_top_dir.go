package analyze

import (
	"os"
	"path/filepath"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

var _ common.Analyzer = (*TopDirAnalyzer)(nil)

// TopDirAnalyzer implements Analyzer
// It doesn't return the full directory structure, only the top level directory,
// thus is suitable only for non-interactive mode.
// It tries to use only stack for storing state and results.
type TopDirAnalyzer struct {
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

// CreateTopDirAnalyzer returns Analyzer
func CreateTopDirAnalyzer() *TopDirAnalyzer {
	return &TopDirAnalyzer{
		progress: &common.CurrentProgress{
			ItemCount: 0,
			TotalSize: int64(0),
		},
		progressChan:     make(chan common.CurrentProgress, 1),
		progressOutChan:  make(chan common.CurrentProgress, 1),
		progressDoneChan: make(chan struct{}),
		doneChan:         make(common.SignalGroup),
		wait:             (&WaitGroup{}).Init(),
	}
}

// SetFollowSymlinks sets whether symlink to files should be followed
func (a *TopDirAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

// SetShowAnnexedSize sets whether to use annexed size of git-annex files
func (a *TopDirAnalyzer) SetShowAnnexedSize(v bool) {
	a.gitAnnexedSize = v
}

// SetTimeFilter sets the time filter function for file inclusion
func (a *TopDirAnalyzer) SetTimeFilter(matchesTimeFilterFn common.TimeFilter) {
	a.matchesTimeFilterFn = matchesTimeFilterFn
}

// SetArchiveBrowsing sets whether browsing of zip/jar/tar archives is enabled
func (a *TopDirAnalyzer) SetArchiveBrowsing(v bool) {
	a.archiveBrowsing = v
}

// SetFileTypeFilter sets the file type filter function
func (a *TopDirAnalyzer) SetFileTypeFilter(filter common.ShouldFileBeIgnored) {
	a.ignoreFileType = filter
}

// GetProgressChan returns channel for getting progress
func (a *TopDirAnalyzer) GetProgressChan() chan common.CurrentProgress {
	return a.progressOutChan
}

// GetDone returns channel for checking when analysis is done
func (a *TopDirAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

// ResetProgress returns progress
func (a *TopDirAnalyzer) ResetProgress() {
	a.progress = &common.CurrentProgress{}
	a.progressChan = make(chan common.CurrentProgress, 1)
	a.progressOutChan = make(chan common.CurrentProgress, 1)
	a.progressDoneChan = make(chan struct{})
	a.doneChan = make(common.SignalGroup)
	a.wait = (&WaitGroup{}).Init()
}

// AnalyzeDir analyzes given path
func (a *TopDirAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

	go a.updateProgress()

	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}

	dir := SimpleDir{
		SimpleFile: SimpleFile{
			Name:      filepath.Base(path),
			Flag:      getDirFlag(err, len(files)),
			IsDir:     true,
			ItemCount: 1,
		},
		Files: make([]SimpleFile, 0, len(files)),
	}

	var topDirs []*TopDir

	for _, f := range files {
		name := f.Name()
		entryPath := filepath.Join(path, name)
		if f.IsDir() {
			if a.ignoreDir(name, entryPath) {
				continue
			}
			topDir := &TopDir{
				Name: name,
				Flag: ' ',
			}
			topDirs = append(topDirs, topDir)
			a.wait.Add(1)
			go func(entryPath string) {
				a.processSubDir(entryPath, topDir)
				a.wait.Done()
			}(entryPath)
		} else {
			var info os.FileInfo
			// Apply file type filter if set
			if a.ignoreFileType != nil && a.ignoreFileType(name) {
				continue // Skip this file
			}

			info, err = f.Info()
			if err != nil {
				log.Print(err.Error())
				dir.Flag = '!'
				continue
			}

			// Apply time filter if set
			if a.matchesTimeFilterFn != nil && !a.matchesTimeFilterFn(info.ModTime()) {
				continue // Skip this file
			}

			if a.followSymlinks && info.Mode()&os.ModeSymlink != 0 {
				infoF, err := followSymlink(entryPath, a.gitAnnexedSize)
				if err != nil {
					log.Print(err.Error())
					dir.Flag = '!'
					continue
				}
				if infoF != nil {
					info = infoF
				}
			}

			file := SimpleFile{
				Name: name,
				Flag: getFlag(info),
				Size: info.Size(),
			}

			file.Usage = getPlatformSpecificUsage(info)

			dir.Files = append(dir.Files, file)
		}
	}

	a.wait.Wait()

	for _, topDir := range topDirs {
		size, usage, itemCount := topDir.GetUsage()
		dir.Files = append(dir.Files, SimpleFile{
			Name:      topDir.Name,
			Flag:      topDir.Flag,
			Size:      size,
			Usage:     usage,
			ItemCount: itemCount,
			IsDir:     true,
		})
	}

	dir.BasePath = filepath.Dir(path)

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return &dir
}

func (a *TopDirAnalyzer) processSubDir(path string, topDir *TopDir) {
	var (
		err        error
		totalSize  int64 = 4096
		totalUsage int64 = 4096
		totalCount int64
		info       os.FileInfo
	)

	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
		topDir.SetFlag('.')
	}

	for _, f := range files {
		name := f.Name()
		entryPath := filepath.Join(path, name)
		if f.IsDir() {
			if a.ignoreDir(name, entryPath) {
				continue
			}

			a.wait.Add(1)
			go func(entryPath string) {
				concurrencyLimit <- struct{}{}

				a.processSubDir(entryPath, topDir)

				<-concurrencyLimit
				a.wait.Done()
			}(entryPath)
		} else {
			// Apply file type filter if set
			if a.ignoreFileType != nil && a.ignoreFileType(name) {
				continue // Skip this file
			}

			totalCount++

			info, err = f.Info()
			if err != nil {
				log.Print(err.Error())
				topDir.SetFlag('.')
				continue
			}

			// Apply time filter if set
			if a.matchesTimeFilterFn != nil && !a.matchesTimeFilterFn(info.ModTime()) {
				continue // Skip this file
			}

			if a.followSymlinks && info.Mode()&os.ModeSymlink != 0 {
				infoF, err := followSymlink(entryPath, a.gitAnnexedSize)
				if err != nil {
					log.Print(err.Error())
					topDir.SetFlag('.')
					continue
				}
				if infoF != nil {
					info = infoF
				}
			}

			totalUsage += getPlatformSpecificUsage(info)
			totalSize += info.Size()
		}
	}

	a.progressChan <- common.CurrentProgress{
		CurrentItemName: path,
		ItemCount:       totalCount,
		TotalSize:       totalUsage,
	}

	topDir.AddUsage(totalSize, totalUsage, totalCount+1)
}

func (a *TopDirAnalyzer) updateProgress() {
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
