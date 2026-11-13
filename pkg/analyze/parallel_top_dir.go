package analyze

import (
	"os"
	"path/filepath"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

var pathSep = string(os.PathSeparator)

var _ common.Analyzer = (*TopDirAnalyzer)(nil)

// TopDirAnalyzer implements Analyzer
// It doesn't return the full directory structure, only the top level directory,
// thus is suitable only for non-interactive mode.
// It tries to use only stack for storing state and results.
type TopDirAnalyzer struct {
	BaseAnalyzer
	links linkTracker
}

// CreateTopDirAnalyzer returns Analyzer
func CreateTopDirAnalyzer() *TopDirAnalyzer {
	a := &TopDirAnalyzer{}
	a.Init()
	return a
}

// FullAnalyzer returns a parallel analyzer with the same settings, for when
// the full tree is needed, which the top dir analyzer does not build.
func (a *TopDirAnalyzer) FullAnalyzer() *ParallelAnalyzer {
	full := CreateAnalyzer()
	full.copySettings(&a.BaseAnalyzer)
	return full
}

// AnalyzeDir analyzes given path
func (a *TopDirAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	a.links.reset()
	a.ignoreDir = ignore
	if fileTypeFilter != nil {
		a.ignoreFileType = fileTypeFilter
	}

	var subDirChan = make(chan struct{})

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
	dir.OwnUsage = dirOwnUsage(path)

	var topDirs []*TopDir

	for _, f := range files {
		if a.IsCancelled() {
			break
		}
		if f.IsDir() {
			name := f.Name()
			if a.shouldSkipDir(name, filepath.Join(path, name)) {
				continue
			}
			topDirs = append(topDirs, &TopDir{Name: name, Flag: ' '})
			continue
		}

		// Files at the top level are listed one by one, so each is measured
		// even when another link to it is counted instead.
		entry, ok, err := a.resolveFile(path, f, resolveOptions{})
		if err != nil {
			log.Print(err.Error())
			dir.Flag = '!'
			continue
		}
		if !ok {
			continue
		}
		file := SimpleFile{
			Name:      entry.name,
			Mtime:     entry.info.ModTime(),
			Flag:      entry.flag,
			Size:      entry.info.Size(),
			Usage:     entry.usage,
			Symlink:   entry.symlink,
			ItemCount: 1,
		}
		if entry.link != (inodeKey{}) {
			file.alreadyCounted = !a.links.claim(entry.link)
		}
		dir.Files = append(dir.Files, file)
	}

	// Register root-level hard links before scanning subdirectories so their
	// storage is counted at the root, while every file keeps its displayed size.
	for _, topDir := range topDirs {
		go func() {
			a.processSubDir(filepath.Join(path, topDir.Name), topDir, true)
			subDirChan <- struct{}{}
		}()
	}

	for i := 0; i < len(topDirs); i++ {
		<-subDirChan
	}

	a.wait.Wait()

	for _, topDir := range topDirs {
		size, usage, itemCount := topDir.GetUsage()
		dir.Files = append(dir.Files, SimpleFile{
			Name:      topDir.Name,
			Mtime:     topDir.Mtime,
			Flag:      topDir.Flag,
			Size:      size,
			Usage:     usage,
			ItemCount: itemCount + 1, // include the directory itself, like Dir does
			IsDir:     true,
		})
	}

	dir.BasePath = filepath.Dir(path)

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return &dir
}

// processSubDir adds the stats of path to topDir. isTopDir is true when path
// is topDir itself rather than one of its subdirectories.
func (a *TopDirAnalyzer) processSubDir(path string, topDir *TopDir, isTopDir bool) {
	var (
		totalSize  int64
		totalUsage int64
		totalCount int64
		mtime      time.Time
	)

	if a.IsCancelled() {
		return
	}

	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}
	switch {
	case isTopDir:
		topDir.SetFlag(getDirFlag(err, len(files)))
	case err != nil:
		topDir.markReadError(false)
	}

	if dirInfo, statErr := os.Stat(path); statErr == nil {
		totalUsage += dirInfoOwnUsage(dirInfo)
		mtime = dirInfo.ModTime()
	}

	for _, f := range files {
		if a.IsCancelled() {
			break
		}
		if f.IsDir() {
			name := f.Name()
			entryPath := path + pathSep + name
			if a.shouldSkipDir(name, entryPath) {
				continue
			}

			totalCount++

			select {
			case concurrencyLimit <- struct{}{}:
				a.wait.Add(1)
				go func(entryPath string) {
					a.processSubDir(entryPath, topDir, false)
					<-concurrencyLimit
					a.wait.Done()
				}(entryPath)
			default:
				a.processSubDir(entryPath, topDir, false)
			}
			continue
		}

		entry, ok, err := a.resolveFile(path, f, resolveOptions{links: &a.links})
		if err != nil {
			log.Print(err.Error())
			topDir.markReadError(isTopDir)
			continue
		}
		if !ok {
			continue
		}

		totalCount++
		if entry.info.ModTime().After(mtime) {
			mtime = entry.info.ModTime()
		}
		if entry.duplicate {
			continue
		}
		totalUsage += entry.usage
		totalSize += entry.info.Size()
	}

	if len(files) == 0 {
		totalSize = EmptyDirSize
	}

	a.progressItemCount.Add(totalCount)
	a.progressTotalUsage.Add(totalUsage)

	topDir.AddUsage(totalSize, totalUsage, totalCount)
	topDir.UpdateMtime(mtime)
}
