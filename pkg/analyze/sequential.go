package analyze

import (
	"os"
	"path/filepath"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

// SequentialAnalyzer implements Analyzer
type SequentialAnalyzer struct {
	BaseAnalyzer
}

// CreateSeqAnalyzer returns Analyzer
func CreateSeqAnalyzer() *SequentialAnalyzer {
	a := &SequentialAnalyzer{}
	a.Init()
	return a
}

// AnalyzeDir analyzes given path
func (a *SequentialAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

	go a.UpdateProgress()
	dir := a.processDir(path)

	dir.BasePath = filepath.Dir(path)
	a.setCurrentDir(dir)

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return dir
}

func (a *SequentialAnalyzer) processDir(path string) *Dir {
	var totalSize int64

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
		if a.IsCancelled() {
			break
		}
		if f.IsDir() {
			name := f.Name()
			entryPath := filepath.Join(path, name)
			if a.shouldSkipDir(name, entryPath) {
				continue
			}
			subdir := a.processDir(entryPath)
			subdir.Parent = dir
			dir.AddFile(subdir)
			continue
		}
		totalSize += a.addFile(dir, path, f)
	}

	a.progressCurrentItemName.Store(path)
	a.progressItemCount.Add(int64(len(files)))
	a.progressTotalUsage.Add(totalSize)
	return dir
}
