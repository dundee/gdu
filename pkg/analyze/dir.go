package analyze

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

var concurrencyLimit = make(chan struct{}, 3*runtime.GOMAXPROCS(0))

// ParallelAnalyzer implements Analyzer
type ParallelAnalyzer struct {
	progress         *common.CurrentProgress
	progressChan     chan common.CurrentProgress
	progressOutChan  chan common.CurrentProgress
	progressDoneChan chan struct{}
	doneChan         common.SignalGroup
	wait             *WaitGroup
	ignoreDir        common.ShouldDirBeIgnored
	followSymlinks   bool
}

// CreateAnalyzer returns Analyzer
func CreateAnalyzer() *ParallelAnalyzer {
	return &ParallelAnalyzer{
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
func (a *ParallelAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

// GetProgressChan returns channel for getting progress
func (a *ParallelAnalyzer) GetProgressChan() chan common.CurrentProgress {
	return a.progressOutChan
}

// GetDone returns channel for checking when analysis is done
func (a *ParallelAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

// ResetProgress returns progress
func (a *ParallelAnalyzer) ResetProgress() {
	a.progress = &common.CurrentProgress{}
	a.progressChan = make(chan common.CurrentProgress, 1)
	a.progressOutChan = make(chan common.CurrentProgress, 1)
	a.progressDoneChan = make(chan struct{})
	a.doneChan = make(common.SignalGroup)
	a.wait = (&WaitGroup{}).Init()
}

// AnalyzeDir analyzes given path
func (a *ParallelAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, constGC bool,
) fs.Item {
	if !constGC {
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
		go manageMemoryUsage(a.doneChan)
	}

	a.ignoreDir = ignore

	go a.updateProgress()
	dir := a.processDir(path)

	dir.BasePath = filepath.Dir(path)
	a.wait.Wait()

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

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
				dir.Flag = '!'
				continue
			}
			if a.followSymlinks && info.Mode()&os.ModeSymlink != 0 {
				err = followSymlink(entryPath, &info)
				if err != nil {
					log.Print(err.Error())
					dir.Flag = '!'
					continue
				}
			}

			file = &File{
				Name:   name,
				Flag:   getFlag(info),
				Size:   info.Size(),
				Parent: dir,
			}
			setPlatformSpecificAttrs(file, info)

			totalSize += info.Size()

			dir.AddFile(file)
		}
	}

	go func() {
		var sub *Dir

		for i := 0; i < dirCount; i++ {
			sub = <-subDirChan
			dir.AddFile(sub)
		}

		a.wait.Done()
	}()

	a.progressChan <- common.CurrentProgress{
		CurrentItemName: path,
		ItemCount:       len(files),
		TotalSize:       totalSize,
	}
	return dir
}

func (a *ParallelAnalyzer) updateProgress() {
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

func followSymlink(path string, f *os.FileInfo) error {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	tInfo, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !tInfo.IsDir() {
		*f = tInfo
	}
	return nil
}
