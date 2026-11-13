package analyze

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
)

var pathSep = string(os.PathSeparator)

var _ common.Analyzer = (*TopDirAnalyzer)(nil)

// TopDirAnalyzer implements Analyzer
// It doesn't return the full directory structure, only the top level directory,
// thus is suitable only for non-interactive mode.
// It keeps only the totals of the directories below the top level.
type TopDirAnalyzer struct {
	BaseAnalyzer
	links linkTracker
}

// CreateTopDirAnalyzer returns Analyzer
func CreateTopDirAnalyzer() *TopDirAnalyzer {
	a := &TopDirAnalyzer{}
	a.Init()
	return a
}

// FullAnalyzer returns a parallel analyzer with the same settings, for when
// the full tree is needed, which the top dir analyzer does not build.
func (a *TopDirAnalyzer) FullAnalyzer() *ParallelAnalyzer {
	full := CreateAnalyzer()
	full.copySettings(&a.BaseAnalyzer)
	return full
}

// AnalyzeDir analyzes given path
func (a *TopDirAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	a.links.reset()
	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

	go a.UpdateProgress()
	root := walkTree[*topDirNode](&a.BaseAnalyzer, &topDirSink{a: a}, path, true)

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return root.dir
}

// topDirNode is a directory of a top-dir scan: the scanned root, listing its
// entries, or a directory below it, keeping only its totals.
type topDirNode struct {
	// dir is set for the root, and totals for the others
	dir    *SimpleDir
	totals *runningTotals
	name   string
}

// topDirSink lists the entries of the scanned root, with the totals of each
// of its subdirectories.
type topDirSink struct {
	a *TopDirAnalyzer
	// mu guards the entries of the root, to which its subdirectories are
	// added by the goroutines walking them
	mu sync.Mutex
}

func (s *topDirSink) openDir(_ *topDirNode, path string, root bool) (*topDirNode, bool) {
	if !root {
		return &topDirNode{name: filepath.Base(path)}, true
	}
	return &topDirNode{dir: &SimpleDir{
		SimpleFile: SimpleFile{Name: filepath.Base(path), IsDir: true, ItemCount: 1},
		BasePath:   filepath.Dir(path),
	}}, true
}

func (s *topDirSink) readDir(node *topDirNode, path string, err error, entries int) {
	mtime, ownUsage := dirStats(path)
	if node.dir == nil {
		node.totals = newRunningTotals(mtime, ownUsage)
		node.totals.setFlag(getDirFlag(err, entries))
		return
	}
	node.dir.Flag = getDirFlag(err, entries)
	node.dir.Mtime, node.dir.OwnUsage = mtime, ownUsage
	node.dir.Files = make([]SimpleFile, 0, entries)
}

func (s *topDirSink) resolveOptions(node *topDirNode) resolveOptions {
	if node.dir != nil {
		// Files of the root are listed one by one, so each is measured even
		// when another link to it is counted instead.
		return resolveOptions{}
	}
	return resolveOptions{links: &s.a.links}
}

func (s *topDirSink) addFile(node *topDirNode, entry *fileEntry) int64 {
	if node.dir == nil {
		usage, size := entry.usage, entry.info.Size()
		if entry.duplicate {
			usage, size = 0, 0
		}
		node.totals.addEntry(1, size, usage, entry.info.ModTime(), entry.flag, false)
		return usage
	}

	file := SimpleFile{
		Name:      entry.name,
		Mtime:     entry.info.ModTime(),
		Flag:      entry.flag,
		Size:      entry.info.Size(),
		Usage:     entry.usage,
		Symlink:   entry.symlink,
		ItemCount: 1,
	}
	if entry.link != (inodeKey{}) {
		file.alreadyCounted = !s.a.links.claim(entry.link)
	}
	s.mu.Lock()
	node.dir.Files = append(node.dir.Files, file)
	s.mu.Unlock()
	if file.alreadyCounted {
		return 0
	}
	return file.Usage
}

func (s *topDirSink) fileError(node *topDirNode) {
	if node.dir == nil {
		node.totals.setFlag('!')
		return
	}
	s.mu.Lock()
	node.dir.Flag = '!'
	s.mu.Unlock()
}

func (s *topDirSink) entriesDone(*topDirNode) {}

func (s *topDirSink) closeDir(*topDirNode) {}

func (s *topDirSink) addDir(parent, child *topDirNode) {
	itemCount, size, usage, mtime, flag := child.totals.result(s.a.isFilteringFiles())
	if parent.dir == nil {
		parent.totals.addEntry(itemCount, size, usage, mtime, flag, true)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	parent.dir.Files = append(parent.dir.Files, SimpleFile{
		Name:      child.name,
		Mtime:     mtime,
		Flag:      flag,
		Size:      size,
		Usage:     usage,
		ItemCount: itemCount,
		IsDir:     true,
	})
}
