package analyze

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// CurrentProgress struct
type CurrentProgress struct {
	Mutex           *sync.Mutex
	CurrentItemName string
	ItemCount       int
	TotalSize       int64
	Done            bool
}

var concurrencyLimit chan struct{} = make(chan struct{}, 2*runtime.NumCPU())

// ShouldDirBeIgnored whether path should be ignored
type ShouldDirBeIgnored func(path string) bool

// Analyzer is type for dir analyzing function
type Analyzer interface {
	AnalyzeDir(path string, ignore ShouldDirBeIgnored) *File
	GetProgress() *CurrentProgress
	ResetProgress()
}

// ParallelAnalyzer implements Analyzer
type ParallelAnalyzer struct {
	progress  *CurrentProgress
	wait      sync.WaitGroup
	ignoreDir ShouldDirBeIgnored
}

// CreateAnalyzer returns Analyzer
func CreateAnalyzer() Analyzer {
	return &ParallelAnalyzer{
		progress: &CurrentProgress{
			Mutex:     &sync.Mutex{},
			Done:      false,
			ItemCount: 0,
			TotalSize: int64(0),
		},
	}
}

// GetProgress returns progress
func (a *ParallelAnalyzer) GetProgress() *CurrentProgress {
	return a.progress
}

// ResetProgress returns progress
func (a *ParallelAnalyzer) ResetProgress() {
	a.progress.Done = false
	a.progress.ItemCount = 0
	a.progress.TotalSize = int64(0)
	a.progress.Mutex = &sync.Mutex{}
}

// AnalyzeDir analyzes given path
func (a *ParallelAnalyzer) AnalyzeDir(path string, ignore ShouldDirBeIgnored) *File {
	a.ignoreDir = ignore
	dir := a.processDir(path)
	dir.BasePath = filepath.Dir(path)
	a.wait.Wait()

	links := make(AlreadyCountedHardlinks, 10)
	dir.UpdateStats(links)

	a.progress.Mutex.Lock()
	a.progress.Done = true
	a.progress.Mutex.Unlock()

	return dir
}

func (a *ParallelAnalyzer) processDir(path string) *File {
	var (
		file      *File
		err       error
		mutex     sync.Mutex
		totalSize int64
		info      os.FileInfo
	)

	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}

	dir := File{
		Name:      filepath.Base(path),
		Flag:      getDirFlag(err, len(files)),
		IsDir:     true,
		ItemCount: 1,
		Files:     make([]*File, 0, len(files)),
	}

	for _, f := range files {
		entryPath := filepath.Join(path, f.Name())
		if f.IsDir() {
			if a.ignoreDir(entryPath) {
				continue
			}

			a.wait.Add(1)
			go func() {
				concurrencyLimit <- struct{}{}
				subdir := a.processDir(entryPath)
				subdir.Parent = &dir

				mutex.Lock()
				dir.Files = append(dir.Files, subdir)
				mutex.Unlock()

				<-concurrencyLimit
				a.wait.Done()
			}()
		} else {
			info, _ = f.Info()
			file = &File{
				Name:      f.Name(),
				Flag:      getFlag(info),
				Size:      info.Size(),
				ItemCount: 1,
				Parent:    &dir,
			}
			setPlatformSpecificAttrs(file, info)

			totalSize += info.Size()

			mutex.Lock()
			dir.Files = append(dir.Files, file)
			mutex.Unlock()
		}
	}

	a.updateProgress(path, len(files), totalSize)
	return &dir
}

func (a *ParallelAnalyzer) updateProgress(path string, itemCount int, totalSize int64) {
	a.progress.Mutex.Lock()
	a.progress.CurrentItemName = path
	a.progress.ItemCount += itemCount
	a.progress.TotalSize += totalSize
	a.progress.Mutex.Unlock()
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
