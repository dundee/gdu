package analyze

import (
	"context"
	"os"
	"path/filepath"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
)

var _ common.Analyzer = (*ParallelAnalyzer)(nil)

// ParallelAnalyzer implements Analyzer
type ParallelAnalyzer struct {
	BaseAnalyzer
}

// CreateAnalyzer returns Analyzer
func CreateAnalyzer() *ParallelAnalyzer {
	a := &ParallelAnalyzer{}
	a.Init()
	return a
}

// AnalyzeDir analyzes given path
func (a *ParallelAnalyzer) AnalyzeDir(ctx context.Context, path string, opts common.ScanOptions) fs.Item {
	return a.analyzeTree(ctx, path, opts, true)
}

// analyzeTree scans path into an in-memory tree, in parallel or in sequence.
func (a *BaseAnalyzer) analyzeTree(ctx context.Context, path string, opts common.ScanOptions, parallel bool) fs.Item {
	s := a.begin(ctx, opts)
	return walkTree[*Dir](a, s, treeSink{a}, path, parallel)
}

// treeSink builds the in-memory tree of a scan. A directory is added to its
// parent once it is complete; the root can be previewed during the scan.
type treeSink struct {
	a *BaseAnalyzer
}

func (s treeSink) openDir(parent *Dir, path string, root bool) (*Dir, bool) {
	dir := &Dir{
		File:      &File{Name: filepath.Base(path)},
		ItemCount: 1,
	}
	if root {
		dir.BasePath = filepath.Dir(path)
		s.a.setCurrentDir(dir)
	} else {
		dir.Parent = parent
	}
	return dir, true
}

func (treeSink) readDir(dir *Dir, path string, err error, entries int) {
	dir.m.Lock()
	defer dir.m.Unlock()
	dir.Flag = getDirFlag(err, entries)
	dir.Files = make(fs.Files, 0, entries)
	dir.Mtime, dir.OwnUsage, dir.Dev = dirStats(path)
}

func (treeSink) resolveOptions(*Dir) resolveOptions {
	return resolveOptions{browseArchives: true}
}

func (treeSink) addFile(dir *Dir, entry *fileEntry) int64 {
	item := entry.item(dir)
	dir.AddFile(item)
	return item.GetUsage()
}

func (treeSink) fileError(dir *Dir) {
	dir.SetFlag('!')
}

func (treeSink) entriesDone(*Dir) {}

func (treeSink) closeDir(*Dir) {}

func (treeSink) addDir(parent, child *Dir) {
	parent.AddFile(child)
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
	if f.Mode()&os.ModeSymlink != 0 || f.Mode()&os.ModeSocket != 0 {
		return '@'
	}
	return ' '
}
