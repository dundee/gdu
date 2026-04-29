package analyze

import (
	"os"
	"path/filepath"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

// ParallelStableOrderAnalyzer implements Analyzer
type ParallelStableOrderAnalyzer struct {
	BaseAnalyzer
}

// CreateStableOrderAnalyzer returns parallel Analyzer which keeps stable order of files
func CreateStableOrderAnalyzer() *ParallelStableOrderAnalyzer {
	a := &ParallelStableOrderAnalyzer{}
	a.Init()
	return a
}

// AnalyzeDir analyzes given path
func (a *ParallelStableOrderAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

	go a.UpdateProgress()
	dir := a.processDir(path)

	dir.BasePath = filepath.Dir(path)
	a.wait.Wait()

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return dir
}

func (a *ParallelStableOrderAnalyzer) processDir(path string) *Dir {
	type indexedItem struct {
		index int
		item  fs.Item
	}

	var (
		file      *File
		err       error
		totalSize int64
		info      os.FileInfo
		itemCount int
		dirCount  int
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

	// Buffer channel to prevent deadlock when sending files synchronously
	itemChan := make(chan indexedItem, len(files))

	for _, f := range files {
		name := f.Name()
		entryPath := filepath.Join(path, name)

		if f.IsDir() {
			if a.ignoreDir(name, entryPath) {
				continue
			}
			currentIndex := itemCount
			itemCount++
			dirCount++

			go func(entryPath string, idx int) {
				concurrencyLimit <- struct{}{}
				subdir := a.processDir(entryPath)
				subdir.Parent = dir

				itemChan <- indexedItem{idx, subdir}
				<-concurrencyLimit
			}(entryPath, currentIndex)
		} else {
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

			// Apply time filter if set
			if a.matchesTimeFilterFn != nil && !a.matchesTimeFilterFn(info.ModTime()) {
				continue // Skip this file
			}

			file = &File{
				Name:   name,
				Flag:   getFlag(info),
				Size:   info.Size(),
				Parent: dir,
			}
			setPlatformSpecificAttrs(file, info)

			totalSize += file.Usage

			// Send file to channel with its index
			itemChan <- indexedItem{itemCount, file}
			itemCount++
		}
	}

	go func() {
		items := make([]indexedItem, itemCount)

		// Collect all items (both files and subdirs)
		for i := 0; i < itemCount; i++ {
			indexed := <-itemChan
			items[indexed.index] = indexed
		}

		// Add all items in their original order
		for i := 0; i < itemCount; i++ {
			dir.AddFile(items[i].item)
		}

		a.wait.Done()
	}()

	a.progressCurrentItemName.Store(path)
	a.progressItemCount.Add(int64(len(files)))
	a.progressTotalUsage.Add(totalSize)
	return dir
}
