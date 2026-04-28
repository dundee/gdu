package analyze

import (
	"os"
	"path/filepath"
	"sync"

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
	BaseAnalyzer
	linkedItems sync.Map
}

// CreateTopDirAnalyzer returns Analyzer
func CreateTopDirAnalyzer() *TopDirAnalyzer {
	a := &TopDirAnalyzer{}
	a.Init()
	return a
}

// AnalyzeDir analyzes given path
func (a *TopDirAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

	go a.UpdateProgress()

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

			usage, mli := getPlatformSpecificUsageAndMli(info)
			file.Usage = usage

			if mli > 0 {
				file.Flag = 'H'
			}

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

			usage, mli := getPlatformSpecificUsageAndMli(info)

			if mli > 0 {
				if _, loaded := a.linkedItems.LoadOrStore(mli, struct{}{}); loaded {
					continue
				}
			}

			totalUsage += usage
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
