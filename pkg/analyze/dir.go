package analyze

import (
	"os"
	"path/filepath"
	"runtime"

	log "github.com/sirupsen/logrus"
)

// CurrentProgress struct
type CurrentProgress struct {
	CurrentItemName string
	ItemCount       int
	TotalSize       int64
}

var concurrencyLimit = make(chan struct{}, 3*runtime.GOMAXPROCS(0))

// ShouldDirBeIgnored whether path should be ignored
type ShouldDirBeIgnored func(name, path string) bool

// Analyzer is type for dir analyzing function
type Analyzer interface {
	AnalyzeDir(path string, ignore ShouldDirBeIgnored) *Dir
	GetProgressChan() chan CurrentProgress
	GetDoneChan() chan struct{}
	ResetProgress()
}

// ParallelAnalyzer implements Analyzer
type ParallelAnalyzer struct {
	progress        *CurrentProgress
	progressChan    chan CurrentProgress
	progressOutChan chan CurrentProgress
	doneChan        chan struct{}
	wait            *WaitGroup
	ignoreDir       ShouldDirBeIgnored
}

// CreateAnalyzer returns Analyzer
func CreateAnalyzer() Analyzer {
	return &ParallelAnalyzer{
		progress: &CurrentProgress{
			ItemCount: 0,
			TotalSize: int64(0),
		},
		progressChan:    make(chan CurrentProgress, 1),
		progressOutChan: make(chan CurrentProgress, 1),
		doneChan:        make(chan struct{}, 1),
		wait:            (&WaitGroup{}).Init(),
	}
}

// GetProgressChan returns channel for getting progress
func (a *ParallelAnalyzer) GetProgressChan() chan CurrentProgress {
	return a.progressOutChan
}

// GetDoneChan returns channel for checking when analysis is done
func (a *ParallelAnalyzer) GetDoneChan() chan struct{} {
	return a.doneChan
}

// ResetProgress returns progress
func (a *ParallelAnalyzer) ResetProgress() {
	a.progress.ItemCount = 0
	a.progress.TotalSize = int64(0)
	a.progress.CurrentItemName = ""
}

// AnalyzeDir analyzes given path
func (a *ParallelAnalyzer) AnalyzeDir(path string, ignore ShouldDirBeIgnored) *Dir {
	a.ignoreDir = ignore

	go a.updateProgress()
	dir := a.processDir(path)

	dir.BasePath = filepath.Dir(path)
	a.wait.Wait()

	a.doneChan <- struct{}{} // finish updateProgress here
	a.doneChan <- struct{}{} // and there

	return dir
}

func (a *ParallelAnalyzer) processDir(path string) *Dir {
	var (
		file       *File
		err        error
		totalSize  int64
		info       os.FileInfo
		subDirChan = make(chan *Dir)
		dirCount   int
	)

	a.wait.Add(1)

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
		Files:     make([]Item, 0, len(files)),
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

			go func(entryPath string) {
				concurrencyLimit <- struct{}{}
				subdir := a.processDir(entryPath)
				subdir.Parent = dir

				subDirChan <- subdir
				<-concurrencyLimit
			}(entryPath)
		} else {
			info, err = f.Info()
			if err != nil {
				log.Print(err.Error())
				continue
			}
			file = &File{
				Name:   name,
				Flag:   getFlag(info),
				Size:   info.Size(),
				Parent: dir,
			}
			setPlatformSpecificAttrs(file, info)

			totalSize += info.Size()

			dir.Files.Append(file)
		}
	}

	go func() {
		var sub *Dir

		for i := 0; i < dirCount; i++ {
			sub = <-subDirChan
			dir.Files.Append(sub)
		}

		a.wait.Done()
	}()

	a.progressChan <- CurrentProgress{path, len(files), totalSize}
	return dir
}

func (a *ParallelAnalyzer) updateProgress() {
	for {
		select {
		case <-a.doneChan:
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

func getDirFlag(err error, items int) rune {
	switch {
	case err != nil:
		return '!'
	case items == 0:
		return 'e'
	default:
		return ' '
	}
}

func getFlag(f os.FileInfo) rune {
	switch {
	case f.Mode()&os.ModeSymlink != 0:
		fallthrough
	case f.Mode()&os.ModeSocket != 0:
		return '@'
	default:
		return ' '
	}
}
