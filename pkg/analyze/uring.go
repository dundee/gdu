package analyze

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/iceber/iouring-go"
	log "github.com/sirupsen/logrus"
)

var statxFlags uint32 = unix.AT_SYMLINK_NOFOLLOW | unix.AT_STATX_DONT_SYNC
var statxMask = unix.STATX_TYPE | unix.STATX_SIZE

// UringAnalyzer is a parallel analyzer with io_uring usage
type UringAnalyzer struct {
	doneChan     common.SignalGroup
	progressChan chan common.CurrentProgress
	ignoreDir    common.ShouldDirBeIgnored
	iour         *iouring.IOURing
	wait         *WaitGroup
}

// CreateUringAnalyzer returns analyzer which uses io_uring
func CreateUringAnalyzer() *UringAnalyzer {
	return &UringAnalyzer{
		doneChan:     make(common.SignalGroup),
		progressChan: make(chan common.CurrentProgress, 1),
		wait:         (&WaitGroup{}).Init(),
	}
}

// GetDone returns channel for checking when analysis is done
func (a *UringAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

// GetProgressChan returns channel for getting progress
func (a *UringAnalyzer) GetProgressChan() chan common.CurrentProgress {
	return a.progressChan
}

// ResetProgress returns progress
func (a *UringAnalyzer) ResetProgress() {
	a.doneChan = make(common.SignalGroup)
	a.wait = (&WaitGroup{}).Init()
}

// AnalyzeDir analyzes given path
func (a *UringAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, constGC bool,
) fs.Item {
	if !constGC {
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
		go manageMemoryUsage(a.doneChan)
	}

	iour, err := iouring.New(32768)
	if err != nil {
		log.Print(err.Error())
	}
	defer iour.Close()
	a.iour = iour

	a.ignoreDir = ignore

	dir := a.processDir(path)

	dir.BasePath = filepath.Dir(path)
	a.wait.Wait()

	a.doneChan.Broadcast()

	return dir
}

func (a *UringAnalyzer) processDir(path string) *Dir {
	var (
		file       *File
		err        error
		totalSize  int64
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
	a.setDirPlatformSpecificAttrs(dir, path)

	stats := make(map[string]*unix.Statx_t)
	var reqs []iouring.PrepRequest

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
			stat := &unix.Statx_t{}
			req, err := iouring.Statx(
				unix.AT_FDCWD, entryPath, statxFlags, statxMask, stat,
			)
			if err != nil {
				log.Print(err.Error())
				continue
			}

			reqs = append(reqs, req)
			stats[name] = stat
		}
	}

	if len(reqs) > 0 {
		res, err := a.iour.SubmitRequests(reqs, nil)
		if err != nil {
			log.Print(err.Error())
			return nil
		}
		<-res.Done()

		for name, stat := range stats {
			file = &File{
				Name:   name,
				Flag:   getFlagStatx(stat),
				Size:   int64(stat.Size),
				Parent: dir,
			}
			a.setPlatformSpecificAttrs(file, stat)

			totalSize += int64(stat.Size)

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

	return dir
}

func (a *UringAnalyzer) setPlatformSpecificAttrs(file *File, stat *unix.Statx_t) {
	file.Usage = int64(stat.Blocks * devBSize)
	file.Mtime = time.Unix(int64(stat.Mtime.Sec), int64(stat.Mtime.Nsec))

	if stat.Nlink > 1 {
		file.Mli = stat.Ino
	}
}

func (a *UringAnalyzer) setDirPlatformSpecificAttrs(dir *Dir, path string) {
	stat := &unix.Statx_t{}
	req, err := iouring.Statx(
		unix.AT_FDCWD, path, statxFlags, statxMask, stat,
	)
	if err != nil {
		log.Print(err.Error())
		return
	}
	res, err := a.iour.SubmitRequest(req, nil)
	if err != nil {
		log.Print(err.Error())
		return
	}
	<-res.Done()

	dir.Mtime = time.Unix(int64(stat.Mtime.Sec), int64(stat.Mtime.Nsec))
}

func getFlagStatx(f *unix.Statx_t) rune {
	fType := f.Mode & syscall.S_IFMT
	switch fType {
	case syscall.S_IFLNK:
		fallthrough
	case syscall.S_IFSOCK:
		return '@'
	default:
		return ' '

	}
}
