package analyze

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

// dirSink is what an analyzer does with the directories a scan walks: build
// a tree of them, store them in a database, or total them. D is the sink's
// handle of a directory.
//
// The walk calls the methods for one directory in order: openDir, readDir,
// addFile and fileError for its files, openDir for its subdirectories,
// entriesDone, and closeDir once its subdirectories are closed, followed by
// addDir for its parent. A directory's files are all added before any of its
// subdirectories is walked. Directories are walked concurrently, so a sink
// must be safe for use by several goroutines, but the calls for one
// directory never overlap, except that addDir for its subdirectories comes
// from the goroutines walking them.
type dirSink[D any] interface {
	// openDir starts the directory at path, an entry of parent, or the
	// scanned root when root is set. It is called by the walk of parent. It
	// returns false to leave the directory out of the scan.
	openDir(parent D, path string, root bool) (D, bool)
	// readDir tells the result of reading the directory: the error, and how
	// many entries it has.
	readDir(dir D, path string, err error, entries int)
	// resolveOptions tells how the files of dir are resolved.
	resolveOptions(dir D) resolveOptions
	// addFile adds a file to dir and returns the disk usage it counted.
	addFile(dir D, entry *fileEntry) int64
	// fileError tells that an entry of dir could not be read.
	fileError(dir D)
	// entriesDone tells that every file of dir has been added and every
	// subdirectory opened.
	entriesDone(dir D)
	// closeDir tells that dir and everything below it has been walked.
	closeDir(dir D)
	// addDir adds child, closed, to its parent.
	addDir(parent, child D)
}

// walker walks the directory tree of one scan and hands what it finds to a
// sink. A parallel walk reads a subdirectory in a goroutine of its own when
// one of a bounded number of slots is free, and in the goroutine meeting it
// otherwise, so it never waits for a slot.
type walker[D any] struct {
	a    *BaseAnalyzer
	scan *scan
	sink dirSink[D]
	// slots bounds the goroutines walking at once; nil walks in sequence
	slots chan struct{}
	// running counts the goroutines walking, so the walk can wait for them
	running sync.WaitGroup
}

// walkNode is a directory being walked. It is closed when its own entries
// and all its subdirectories are done, by whichever goroutine finishes last.
type walkNode[D any] struct {
	dir    D
	name   string
	parent *walkNode[D]
	// pending counts the directory's own entries, as one, and its
	// subdirectories not closed yet
	pending atomic.Int64
}

// walkTree walks the tree at path for the scan s with sink, in parallel or in
// sequence, and returns the sink's handle of path. Progress is reported on a
// as it goes.
func walkTree[D any](a *BaseAnalyzer, s *scan, sink dirSink[D], path string, parallel bool) D {
	w := &walker[D]{a: a, scan: s, sink: sink}
	if parallel {
		// sized now rather than at start-up, so that it follows --max-cores
		w.slots = make(chan struct{}, 2*runtime.GOMAXPROCS(0))
	}
	var zero D
	root, ok := sink.openDir(zero, path, true)
	if !ok {
		return zero
	}
	w.walkDir(&walkNode[D]{dir: root}, path)
	w.running.Wait()
	return root
}

// walkDir walks the directory of node, at path.
func (w *walker[D]) walkDir(node *walkNode[D], path string) {
	node.pending.Store(1)

	entries, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}
	w.sink.readDir(node.dir, path, err, len(entries))
	opts := w.sink.resolveOptions(node.dir)

	var (
		usage, items int64
		subdirs      []string
	)
	for _, entry := range entries {
		if w.scan.isCancelled() {
			break
		}
		if entry.IsDir() {
			subdirs = append(subdirs, entry.Name())
			continue
		}

		file, ok, err := w.scan.resolveFile(path, entry, opts)
		if err != nil {
			log.Print(err.Error())
			w.sink.fileError(node.dir)
			continue
		}
		if !ok {
			continue
		}
		usage += w.sink.addFile(node.dir, &file)
		if file.archive != nil {
			items += file.archive.GetItemCount()
		} else {
			items++
		}
	}

	children := make([]*walkNode[D], 0, len(subdirs))
	for _, name := range subdirs {
		subPath := filepath.Join(path, name)
		if w.scan.shouldSkipDir(name, subPath) {
			continue
		}
		if sub, ok := w.sink.openDir(node.dir, subPath, false); ok {
			children = append(children, &walkNode[D]{dir: sub, name: name, parent: node})
		}
	}
	w.sink.entriesDone(node.dir)

	w.a.progressCurrentItemName.Store(path)
	w.a.progressItemCount.Add(items + 1)
	w.a.progressTotalUsage.Add(usage)

	node.pending.Add(int64(len(children)))
	for _, child := range children {
		w.walkSubdir(child, filepath.Join(path, child.name))
	}
	w.done(node)
}

// walkSubdir walks the subdirectory of node in a goroutine of its own when a
// slot is free, and in this one otherwise.
func (w *walker[D]) walkSubdir(node *walkNode[D], path string) {
	select {
	case w.slots <- struct{}{}:
		w.running.Go(func() {
			defer func() { <-w.slots }()
			w.walkDir(node, path)
		})
	default:
		w.walkDir(node, path)
	}
}

// done marks one of node's pending parts done, and closes the directory, and
// then its parents in turn, when nothing of it is pending any more.
func (w *walker[D]) done(node *walkNode[D]) {
	for node != nil && node.pending.Add(-1) == 0 {
		w.sink.closeDir(node.dir)
		if node.parent != nil {
			w.sink.addDir(node.parent.dir, node.dir)
		}
		node = node.parent
	}
}

// dirStats returns the mtime of the directory at path, the disk usage of the
// blocks allocated to the directory itself, and its device.
func dirStats(path string) (mtime time.Time, ownUsage int64, dev uint64) {
	info, err := os.Stat(path)
	if err != nil {
		return mtime, 0, 0
	}
	return info.ModTime(), dirInfoOwnUsage(info), dirInfoDevice(info)
}

// runningTotals are the totals of a directory added up as a scan meets its
// entries, for the analyzers that total as they scan. They come out as
// aggregateDirEntries computes them for a tree after the scan.
type runningTotals struct {
	mu     sync.Mutex
	totals dirTotals
	mtime  time.Time
	flag   rune
}

func newRunningTotals(mtime time.Time, ownUsage int64) *runningTotals {
	return &runningTotals{totals: dirTotals{itemCount: 1, ownUsage: ownUsage}, mtime: mtime, flag: ' '}
}

// setFlag sets the flag the directory has of its own, from reading it.
func (t *runningTotals) setFlag(flag rune) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.flag != '!' {
		t.flag = flag
	}
}

// addEntry adds an entry with its totals.
func (t *runningTotals) addEntry(itemCount, size, usage int64, mtime time.Time, flag rune, isDir bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totals.entries++
	t.totals.itemCount += itemCount
	t.totals.size += size
	t.totals.usage += usage
	if !isDir {
		t.totals.hasFiles = true
	}
	if mtime.After(t.mtime) {
		t.mtime = mtime
	}
	if (flag == '!' || flag == '.') && t.flag != '!' {
		t.flag = '.'
	}
}

// result returns the totals of the directory.
func (t *runningTotals) result(filteringFiles bool) (itemCount, size, usage int64, mtime time.Time, flag rune) {
	t.mu.Lock()
	defer t.mu.Unlock()
	itemCount, size, usage = resolveDirStats(t.totals, filteringFiles)
	return itemCount, size, usage, t.mtime, t.flag
}
