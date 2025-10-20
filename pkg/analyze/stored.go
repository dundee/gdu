package analyze

import (
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

// StoredAnalyzer implements Analyzer
type StoredAnalyzer struct {
	storage          *Storage
	progress         *common.CurrentProgress
	progressChan     chan common.CurrentProgress
	progressOutChan  chan common.CurrentProgress
	progressDoneChan chan struct{}
	doneChan         common.SignalGroup
	wait             *WaitGroup
	ignoreDir        common.ShouldDirBeIgnored
	storagePath      string
	followSymlinks   bool
	gitAnnexedSize   bool
}

// CreateStoredAnalyzer returns Analyzer
func CreateStoredAnalyzer(storagePath string) *StoredAnalyzer {
	return &StoredAnalyzer{
		storagePath: storagePath,
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

// GetProgressChan returns channel for getting progress
func (a *StoredAnalyzer) GetProgressChan() chan common.CurrentProgress {
	return a.progressOutChan
}

// GetDone returns channel for checking when analysis is done
func (a *StoredAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

func (a *StoredAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

func (a *StoredAnalyzer) SetShowAnnexedSize(v bool) {
	a.gitAnnexedSize = v
}

// ResetProgress returns progress
func (a *StoredAnalyzer) ResetProgress() {
	a.progress = &common.CurrentProgress{}
	a.progressChan = make(chan common.CurrentProgress, 1)
	a.progressOutChan = make(chan common.CurrentProgress, 1)
	a.progressDoneChan = make(chan struct{})
	a.doneChan = make(common.SignalGroup)
	a.wait = (&WaitGroup{}).Init()
}

// AnalyzeDir analyzes given path
func (a *StoredAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, constGC bool,
) fs.Item {
	if !constGC {
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
		go manageMemoryUsage(a.doneChan)
	}

	a.storage = NewStorage(a.storagePath, path)
	closeFn := a.storage.Open()
	defer func() {
		// nasty hack to close storage after all goroutines are done
		// Wait returns immediately if value is 0
		// few last goroutines might still start after that
		time.Sleep(1 * time.Second)
		closeFn()
	}()

	a.ignoreDir = ignore

	go a.updateProgress()
	dir := a.processDir(path)

	a.wait.Wait()

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return dir
}

func (a *StoredAnalyzer) processDir(path string) *StoredDir {
	var (
		file      *File
		err       error
		totalSize int64
		info      os.FileInfo
		dirCount  int
	)

	a.wait.Add(1)

	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}

	dir := &StoredDir{
		Dir: &Dir{
			File: &File{
				Name: filepath.Base(path),
				Flag: getDirFlag(err, len(files)),
			},
			BasePath:  filepath.Dir(path),
			ItemCount: 1,
			Files:     make(fs.Files, 0, len(files)),
		},
	}
	parent := &ParentDir{Path: path}

	setDirPlatformSpecificAttrs(dir.Dir, path)

	for _, f := range files {
		name := f.Name()
		entryPath := filepath.Join(path, name)
		if f.IsDir() {
			if a.ignoreDir(name, entryPath) {
				continue
			}
			dirCount++

			subdir := &StoredDir{
				&Dir{
					File: &File{
						Name: name,
					},
					BasePath: path,
				},
				nil,
				sync.Mutex{},
			}
			dir.AddFile(subdir)

			go func(entryPath string) {
				concurrencyLimit <- struct{}{}
				a.processDir(entryPath)
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
				Parent: parent,
			}
			setPlatformSpecificAttrs(file, info)

			totalSize += info.Size()

			dir.AddFile(file)
		}
	}

	err = a.storage.StoreDir(dir)
	if err != nil {
		log.Print(err.Error())
	}

	a.progressChan <- common.CurrentProgress{
		CurrentItemName: path,
		ItemCount:       len(files),
		TotalSize:       totalSize,
	}

	a.wait.Done()
	return dir
}

func (a *StoredAnalyzer) updateProgress() {
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

// StoredDir implements Dir item stored on disk
type StoredDir struct {
	*Dir
	cachedFiles fs.Files
	dbLock      sync.Mutex
}

// GetParent returns parent dir
func (f *StoredDir) GetParent() fs.Item {
	if DefaultStorage.GetTopDir() == f.GetPath() {
		return nil
	}

	if !DefaultStorage.IsOpen() {
		closeFn := DefaultStorage.Open()
		defer closeFn()
	}

	dir, err := DefaultStorage.GetDirForPath(f.BasePath)
	if err != nil {
		log.Print(err.Error())
	}
	return dir
}

// GetFiles returns files in directory
// If files are already cached, return them
// Otherwise load them from storage
func (f *StoredDir) GetFiles() fs.Files {
	if f.cachedFiles != nil {
		return f.cachedFiles
	}

	if !DefaultStorage.IsOpen() {
		f.dbLock.Lock()
		defer f.dbLock.Unlock()
		closeFn := DefaultStorage.Open()
		defer closeFn()
	}

	var files fs.Files
	for _, file := range f.Files {
		if file.IsDir() {
			dir := &StoredDir{
				&Dir{
					File: &File{
						Name: file.GetName(),
					},
					BasePath: f.GetPath(),
				},
				nil,
				sync.Mutex{},
			}

			err := DefaultStorage.LoadDir(dir)
			if err != nil {
				log.Print(err.Error())
			}
			files = append(files, dir)
		} else {
			files = append(files, file)
		}
	}

	f.cachedFiles = files
	return files
}

// SetFiles sets files in directory
func (f *StoredDir) SetFiles(files fs.Files) {
	f.Files = files
}

// RemoveFile removes file from stored directory
// It also updates size and item count of parent directories
func (f *StoredDir) RemoveFile(item fs.Item) {
	if !DefaultStorage.IsOpen() {
		f.dbLock.Lock()
		defer f.dbLock.Unlock()
		closeFn := DefaultStorage.Open()
		defer closeFn()
	}

	f.SetFiles(f.GetFiles().Remove(item))
	f.cachedFiles = nil

	cur := f
	for {
		cur.ItemCount -= item.GetItemCount()
		cur.Size -= item.GetSize()
		cur.Usage -= item.GetUsage()

		err := DefaultStorage.StoreDir(cur)
		if err != nil {
			log.Print(err.Error())
		}

		parent := cur.GetParent()
		if parent == nil {
			break
		}
		cur = parent.(*StoredDir)
	}
}

// GetItemStats returns item count, apparent usage and real usage of this dir
func (f *StoredDir) GetItemStats(linkedItems fs.HardLinkedItems) (itemCount int, size, usage int64) {
	f.UpdateStats(linkedItems)
	return f.ItemCount, f.GetSize(), f.GetUsage()
}

// UpdateStats recursively updates size and item count
func (f *StoredDir) UpdateStats(linkedItems fs.HardLinkedItems) {
	if !DefaultStorage.IsOpen() {
		closeFn := DefaultStorage.Open()
		defer closeFn()
	}

	totalSize := int64(4096)
	totalUsage := int64(4096)
	var itemCount int
	f.cachedFiles = nil
	for _, entry := range f.GetFiles() {
		count, size, usage := entry.GetItemStats(linkedItems)
		totalSize += size
		totalUsage += usage
		itemCount += count

		if entry.GetMtime().After(f.Mtime) {
			f.Mtime = entry.GetMtime()
		}

		switch entry.GetFlag() {
		case '!', '.':
			if f.Flag != '!' {
				f.Flag = '.'
			}
		}
	}
	f.cachedFiles = nil
	f.ItemCount = itemCount + 1
	f.Size = totalSize
	f.Usage = totalUsage
	err := DefaultStorage.StoreDir(f)
	if err != nil {
		log.Print(err.Error())
	}
}

// ParentDir represents parent directory of single file
// It is used to get path to parent directory of a file
type ParentDir struct {
	Path string
}

func (p *ParentDir) GetPath() string {
	return p.Path
}
func (p *ParentDir) GetName() string                                  { panic("must not be called") }
func (p *ParentDir) GetFlag() rune                                    { panic("must not be called") }
func (p *ParentDir) IsDir() bool                                      { panic("must not be called") }
func (p *ParentDir) GetSize() int64                                   { panic("must not be called") }
func (p *ParentDir) GetType() string                                  { panic("must not be called") }
func (p *ParentDir) GetUsage() int64                                  { panic("must not be called") }
func (p *ParentDir) GetMtime() time.Time                              { panic("must not be called") }
func (p *ParentDir) GetItemCount() int                                { panic("must not be called") }
func (p *ParentDir) GetParent() fs.Item                               { panic("must not be called") }
func (p *ParentDir) SetParent(fs.Item)                                { panic("must not be called") }
func (p *ParentDir) GetMultiLinkedInode() uint64                      { panic("must not be called") }
func (p *ParentDir) EncodeJSON(writer io.Writer, topLevel bool) error { panic("must not be called") }
func (p *ParentDir) UpdateStats(linkedItems fs.HardLinkedItems)       { panic("must not be called") }
func (p *ParentDir) AddFile(fs.Item)                                  { panic("must not be called") }
func (p *ParentDir) GetFiles() fs.Files                               { panic("must not be called") }
func (p *ParentDir) GetFilesLocked() fs.Files                         { panic("must not be called") }
func (p *ParentDir) RLock() func()                                    { panic("must not be called") }
func (p *ParentDir) SetFiles(fs.Files)                                { panic("must not be called") }
func (p *ParentDir) RemoveFile(item fs.Item)                          { panic("must not be called") }
func (p *ParentDir) GetItemStats(
	linkedItems fs.HardLinkedItems,
) (itemCount int, size, usage int64) {
	panic("must not be called")
}
