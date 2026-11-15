package analyze

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/dundee/gdu/v5/pkg/fs"
)

var _ Analyzer = (*ParallelAnalyzer)(nil)

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
func (a *ParallelAnalyzer) AnalyzeDir(ctx context.Context, path string, opts Options) fs.Item {
	return a.analyzeTree(ctx, path, opts, true)
}

// analyzeTree scans path into an in-memory tree, in parallel or in sequence.
func (a *BaseAnalyzer) analyzeTree(ctx context.Context, path string, opts Options, parallel bool) fs.Item {
	s := a.begin(ctx, opts)
	return walkTree[*Dir](a, s, newTreeSink(a), path, parallel)
}

// treeSink builds the in-memory tree of a scan. A directory is added to its
// parent once it is complete; the root can be previewed during the scan.
type treeSink struct {
	a       *BaseAnalyzer
	preview *preview
}

func newTreeSink(a *BaseAnalyzer) *treeSink {
	return &treeSink{a: a, preview: &preview{}}
}

// shared returns the lock to hold while changing dir: the preview's for the
// root, which the preview reads, and none for any other directory, which is
// changed only by the goroutine walking it until it is complete.
func (s *treeSink) shared(dir *Dir) sync.Locker {
	if dir == s.preview.root {
		return &s.preview.mu
	}
	return noLock{}
}

func (s *treeSink) openDir(parent *Dir, path string, root bool) (*Dir, bool) {
	dir := &Dir{
		File:      &File{Name: filepath.Base(path)},
		ItemCount: 1,
	}
	if root {
		dir.BasePath = filepath.Dir(path)
		s.preview.root = dir
		s.a.setCurrentDir(s.preview)
	} else {
		dir.Parent = parent
	}
	return dir, true
}

func (s *treeSink) readDir(dir *Dir, path string, err error, entries int) {
	mtime, ownUsage, dev := dirStats(path)
	lock := s.shared(dir)
	lock.Lock()
	defer lock.Unlock()
	dir.Flag = getDirFlag(err, entries)
	dir.Files = make(fs.Files, 0, entries)
	dir.Mtime, dir.OwnUsage, dir.Dev = mtime, ownUsage, dev
}

func (*treeSink) resolveOptions(*Dir) resolveOptions {
	return resolveOptions{browseArchives: true}
}

func (s *treeSink) addFile(dir *Dir, entry *fileEntry) int64 {
	item := entry.item(dir)
	lock := s.shared(dir)
	lock.Lock()
	defer lock.Unlock()
	dir.AddFile(item)
	return item.GetUsage()
}

func (s *treeSink) fileError(dir *Dir) {
	lock := s.shared(dir)
	lock.Lock()
	defer lock.Unlock()
	dir.SetFlag('!')
}

func (*treeSink) entriesDone(*Dir) {}

func (*treeSink) closeDir(*Dir) {}

// addDir attaches child to its parent, which the goroutines closing its
// subdirectories may do at once.
func (s *treeSink) addDir(parent, child *Dir) {
	s.preview.mu.Lock()
	defer s.preview.mu.Unlock()
	parent.AddFile(child)
}

// noLock is the lock of what no one shares.
type noLock struct{}

func (noLock) Lock()   {}
func (noLock) Unlock() {}

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
