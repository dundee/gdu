package analyze

import (
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

// SequentialAnalyzer implements Analyzer
type SequentialAnalyzer struct {
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

// CreateSeqAnalyzer returns Analyzer
func CreateSeqAnalyzer() *SequentialAnalyzer {
	return &SequentialAnalyzer{
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
func (a *SequentialAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

// SetShowAnnexedSize sets whether to use annexed size of git-annex files
func (a *SequentialAnalyzer) SetShowAnnexedSize(v bool) {
	a.gitAnnexedSize = v
}

// SetTimeFilter sets the time filter function for file inclusion
func (a *SequentialAnalyzer) SetTimeFilter(matchesTimeFilterFn common.TimeFilter) {
	a.matchesTimeFilterFn = matchesTimeFilterFn
}

// SetArchiveBrowsing sets whether browsing of zip/jar archives is enabled
func (a *SequentialAnalyzer) SetArchiveBrowsing(v bool) {
	a.archiveBrowsing = v
}

// SetFileTypeFilter sets the file type filter function
func (a *SequentialAnalyzer) SetFileTypeFilter(filter common.ShouldFileBeIgnored) {
	a.ignoreFileType = filter
}

// GetProgressChan returns channel for getting progress
func (a *SequentialAnalyzer) GetProgressChan() chan common.CurrentProgress {
	return a.progressOutChan
}

// GetDone returns channel for checking when analysis is done
func (a *SequentialAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

// ResetProgress returns progress
func (a *SequentialAnalyzer) ResetProgress() {
	a.progress = &common.CurrentProgress{}
	a.progressChan = make(chan common.CurrentProgress, 1)
	a.progressOutChan = make(chan common.CurrentProgress, 1)
	a.progressDoneChan = make(chan struct{})
	a.doneChan = make(common.SignalGroup)
}

// AnalyzeDir analyzes given path
func (a *SequentialAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored, constGC bool,
) fs.Item {
	if !constGC {
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
		go manageMemoryUsage(a.doneChan)
	}

	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

	go a.updateProgress()
	dir := a.processDir(path)

	dir.BasePath = filepath.Dir(path)

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return dir
}

func (a *SequentialAnalyzer) processDir(path string) *Dir {
	var (
		file      fs.Item
		err       error
		totalSize int64
		info      os.FileInfo
		dirCount  int
	)

	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}

	dir := &Dir{
		File: &File{
			Name: filepath.Base(path),
			Flag: getDirFlag(err, len(files)),
		},
		ItemCount: 1,
		Files:     make(fs.Files, 0, len(files)),
	}
	setDirPlatformSpecificAttrs(dir, path)

	for _, f := range files {
		name := f.Name()
		entryPath := filepath.Join(path, name)
		if f.IsDir() {
			if a.ignoreDir(name, entryPath) {
				continue
			}
			dirCount++

			subdir := a.processDir(entryPath)
			subdir.Parent = dir
			dir.AddFile(subdir)
		} else {
			info, err = f.Info()
			if err != nil {
				log.Print(err.Error())
				dir.Flag = '!'
				continue
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

			// Check if it's a zip or jar file
			if a.archiveBrowsing && isZipFile(name) {
				zipDir, err := processZipFile(entryPath, info)
				if err != nil {
					// If unable to process zip file, treat as regular file
					log.Printf("Failed to process zip file %s: %v", entryPath, err)
					file = &File{
						Name:   name,
						Flag:   getFlag(info),
						Size:   info.Size(),
						Parent: dir,
					}
				} else {
					// Successfully processed zip file, use zip content size
					uncompressedSize, compressedSize, err := getZipFileSize(entryPath)
					if err == nil {
						zipDir.Size = uncompressedSize
						zipDir.Usage = compressedSize
					}
					zipDir.Parent = dir
					file = zipDir
				}
			} else {
				file = &File{
					Name:   name,
					Flag:   getFlag(info),
					Size:   info.Size(),
					Parent: dir,
				}
			}

			// Apply time filter if set
			if a.matchesTimeFilterFn != nil && !a.matchesTimeFilterFn(info.ModTime()) {
				continue // Skip this file
			}

			// Apply file type filter if set
			if a.ignoreFileType != nil && a.ignoreFileType(name) {
				continue // Skip this file
			}

			if file != nil {
				// Only set platform-specific attributes for regular files
				if regularFile, ok := file.(*File); ok {
					setPlatformSpecificAttrs(regularFile, info)
				}
				totalSize += file.GetSize()
				dir.AddFile(file)
			}
		}
	}

	a.progressChan <- common.CurrentProgress{
		CurrentItemName: path,
		ItemCount:       len(files),
		TotalSize:       totalSize,
	}
	return dir
}

func (a *SequentialAnalyzer) updateProgress() {
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
