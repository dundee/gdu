package analyze

import (
	"io/ioutil"
	"log"
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

// ShouldDirBeIgnored whether path should be ignored
type ShouldDirBeIgnored func(path string) bool

// Analyzer is type for dir analyzing funcion
type Analyzer func(path string, progress *CurrentProgress, ignore ShouldDirBeIgnored) *File

// ProcessDir analyzes given path
func ProcessDir(path string, progress *CurrentProgress, ignore ShouldDirBeIgnored) *File {
	concurrencyLimitChannel := make(chan bool, 2*runtime.NumCPU())
	var wait sync.WaitGroup
	dir := processDir(path, progress, concurrencyLimitChannel, &wait, ignore)
	dir.BasePath = filepath.Dir(path)
	wait.Wait()
	dir.UpdateStats()

	progress.Mutex.Lock()
	progress.Done = true
	progress.Mutex.Unlock()

	return dir
}

func processDir(path string, progress *CurrentProgress, concurrencyLimitChannel chan bool, wait *sync.WaitGroup, ignoreDir ShouldDirBeIgnored) *File {
	var file *File
	var err error

	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}

	dir := File{
		Name:      filepath.Base(path),
		IsDir:     true,
		ItemCount: 1,
		Files:     make([]*File, 0, len(files)),
	}

	var mutex sync.Mutex
	var totalSize int64

	for _, f := range files {
		entryPath := filepath.Join(path, f.Name())

		if f.IsDir() {
			if ignoreDir(entryPath) {
				continue
			}

			wait.Add(1)
			go func() {
				concurrencyLimitChannel <- true
				subdir := processDir(entryPath, progress, concurrencyLimitChannel, wait, ignoreDir)
				subdir.Parent = &dir
				mutex.Lock()
				dir.Files = append(dir.Files, subdir)
				mutex.Unlock()
				<-concurrencyLimitChannel
				wait.Done()
			}()
		} else {
			file = &File{
				Name:      f.Name(),
				Size:      f.Size(),
				ItemCount: 1,
				Parent:    &dir,
			}
			totalSize += f.Size()

			mutex.Lock()
			dir.Files = append(dir.Files, file)
			mutex.Unlock()
		}
	}

	progress.Mutex.Lock()
	progress.CurrentItemName = path
	progress.ItemCount += len(files)
	progress.TotalSize += totalSize
	progress.Mutex.Unlock()

	return &dir
}
