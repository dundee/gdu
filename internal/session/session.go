// Package session runs the use cases all gdu frontends share: scanning one or
// several paths, rescanning a directory, loading an exported analysis and
// opening an analysis stored with --db.
//
// A Session owns the resulting tree and the hard-link index its totals are
// computed with, so every frontend gets the same answer to what a scan, a
// rescan or a load means; the frontends only present it.
package session

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// Phase is the stage a running scan is in.
type Phase uint8

const (
	// Scanning means the filesystem is being walked.
	Scanning Phase = iota
	// Finalizing means the walk is over and the totals are being computed.
	Finalizing
)

// Progress is what an Observer is told about a running scan.
type Progress struct {
	common.CurrentProgress
	Phase Phase
	// Root is the number of the scanned root being walked, counted from 1,
	// and Roots is how many scanned roots there are.
	Root, Roots int
}

// Observer is told the progress of a running scan periodically, from a
// goroutine of its own. It is not called any more once the scan has returned.
type Observer func(Progress)

// progressInterval is how often an Observer is told the progress.
var progressInterval = 100 * time.Millisecond

// ErrNoDatabase is returned by Open when the analyzer does not store its
// results in a database.
var ErrNoDatabase = errors.New("reading a stored analysis needs a database (--db)")

// storedAnalyzer is an analyzer storing its results in a database (--db),
// from which the analysis of an earlier scan can be loaded.
type storedAnalyzer interface {
	LoadAnalysis(path string) (fs.Item, error)
}

// Session is the analysis a frontend presents, and every way of producing or
// refreshing it. All its methods are safe to call from any goroutine.
//
// The session owns its tree: whoever reads the tree holds TreeLock for
// reading, and whoever changes it, holds it for writing. A tree being built
// by a scan or a load is not the session's until it is complete.
type Session struct {
	settings *common.UI

	tree sync.RWMutex

	// mu guards the fields below
	mu    sync.RWMutex
	root  fs.Item
	links *fs.HardLinks
	// the analyzer, phase and root number of the running scan, as reported
	// to observers
	analyzer common.Analyzer
	phase    Phase
	current  int
}

// New returns a session scanning with the analyzer and the ignore rules of
// settings, taken as they are when each scan starts.
func New(settings *common.UI) *Session {
	return &Session{settings: settings}
}

// Root returns the top of the tree: a scanned root, or the virtual top level
// dir holding several of them. It is nil until the first scan or load.
func (s *Session) Root() fs.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

// RootPath returns the path of the top of the tree, or "" when there is none.
func (s *Session) RootPath() string {
	if root := s.Root(); root != nil {
		return root.GetPath()
	}
	return ""
}

// RootPaths returns the paths of the scanned roots. It reads the tree holding
// TreeLock, which the caller must not hold.
func (s *Session) RootPaths() []string {
	s.tree.RLock()
	defer s.tree.RUnlock()
	return analyze.ScannedRootPaths(s.Root())
}

// TreeLock returns the lock guarding the tree of the session. Readers of the
// tree hold it for reading; whoever changes the tree, such as a
// remove.Deleter given it with remove.WithTreeLock, holds it for writing.
// Methods of the session that take it say so.
func (s *Session) TreeLock() *sync.RWMutex {
	return &s.tree
}

// HardLinks returns the links to the file item, item included, as counted by
// the totals of the tree, or none when item has a single link.
func (s *Session) HardLinks(item fs.Item) fs.Files {
	key, ok := analyze.HardLinkKey(item)
	if !ok {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.links.Links(key)
}

// SetRoot makes root, an analysis made by other means, the tree of the
// session as it is.
func (s *Session) SetRoot(root fs.Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = root
	s.links = nil
}

// Scan scans paths, computes the totals and makes the result the tree of the
// session, which it returns. Several paths are grouped under the virtual top
// level dir. Cancelling ctx stops the scan, which then keeps what was found so
// far and skips the roots it had not started yet.
func (s *Session) Scan(ctx context.Context, paths []string, observe Observer) fs.Item {
	if len(paths) == 0 {
		return nil
	}
	stop := s.watch(observe, len(paths))
	defer stop()

	roots := make([]fs.Item, 0, len(paths))
	for i, path := range paths {
		root, ok := s.walk(ctx, path, i+1)
		if !ok {
			break
		}
		roots = append(roots, root)
	}

	root := roots[0]
	if len(paths) > 1 {
		root = analyze.CreateVirtualRootDir(roots...)
	}
	// the tree can be previewed already
	s.tree.Lock()
	defer s.tree.Unlock()
	s.finalize(root, s.settings.IsFilteringFiles())
	return root
}

// Rescan scans dir, a directory of the tree, again and puts the result in its
// place. It returns what the tree now holds at dir's place: the rescanned
// directory, or the new top when the whole tree had to be scanned again.
// Cancelling ctx stops the scan as it does for Scan.
//
// It changes the tree holding TreeLock, which the caller must not hold.
func (s *Session) Rescan(ctx context.Context, dir fs.Item, observe Observer) fs.Item {
	parent := dir.GetParent()
	if parent == nil {
		s.tree.RLock()
		paths := analyze.ScannedRootPaths(dir)
		s.tree.RUnlock()
		return s.Scan(ctx, paths, observe)
	}
	if _, ok := s.settings.Analyzer.(storedAnalyzer); ok {
		// a database holds the analysis of whole scanned roots, so it cannot
		// take the scan of a part of one
		return s.Scan(ctx, s.RootPaths(), observe)
	}

	stop := s.watch(observe, 1)
	defer stop()

	scanned, _ := s.walk(ctx, dir.GetPath(), 1)

	s.tree.Lock()
	defer s.tree.Unlock()
	scanned.SetParent(parent)
	// replace dir itself: under the virtual top level dir, scanned roots can
	// share a name
	parent.RemoveFile(dir)
	parent.AddFile(scanned)

	top := parent
	for top.GetParent() != nil {
		top = top.GetParent()
	}
	s.finalize(top, s.settings.IsFilteringFiles())
	return scanned
}

// Load reads an analysis exported as JSON and makes it the tree of the
// session.
func (s *Session) Load(input io.Reader) (fs.Item, error) {
	dir, err := analyze.ReadAnalysis(input)
	if err != nil {
		return nil, err
	}
	// the decoded document is garbage now, and can be much larger than the tree
	runtime.GC()

	// the filters apply to scanning, not to what an export holds
	s.tree.Lock()
	defer s.tree.Unlock()
	s.finalize(dir, false)
	return dir, nil
}

// Open makes the analysis of path stored in the database by an earlier scan
// the tree of the session. It fails with ErrNoDatabase when the analyzer has
// no database.
func (s *Session) Open(path string) (fs.Item, error) {
	stored, ok := s.settings.Analyzer.(storedAnalyzer)
	if !ok {
		return nil, ErrNoDatabase
	}
	root, err := stored.LoadAnalysis(path)
	if err != nil {
		return nil, err
	}
	// the stored totals are final
	s.SetRoot(root)
	return root, nil
}

// Snapshot returns a copy of the tree the running scan has built so far, or
// nil when there is none or the analyzer cannot provide it.
//
// It reads holding TreeLock, which the caller must not hold, as the tree may
// be the session's already.
func (s *Session) Snapshot() fs.Item {
	if analyzer, ok := s.settings.Analyzer.(interface{ GetCurrentDir() fs.Item }); ok {
		s.tree.RLock()
		defer s.tree.RUnlock()
		return analyzer.GetCurrentDir()
	}
	return nil
}

// walk scans path as the scanned root numbered number. It reports false,
// without scanning, when ctx was cancelled before it got to path. The first
// root is always walked, even if only to stop at once, so that a scan always
// ends with a tree.
func (s *Session) walk(ctx context.Context, path string, number int) (fs.Item, bool) {
	if number > 1 && ctx.Err() != nil {
		return nil, false
	}

	analyzer := s.settings.Analyzer
	s.mu.Lock()
	s.analyzer, s.phase, s.current = analyzer, Scanning, number
	s.mu.Unlock()

	return analyzer.AnalyzeDir(ctx, path, s.settings.ScanOptions()), true
}

// finalize computes the totals of the tree topped by root, with a fresh
// hard-link index, and makes it the tree of the session. The caller holds
// TreeLock for writing.
func (s *Session) finalize(root fs.Item, filtering bool) {
	s.mu.Lock()
	s.phase = Finalizing
	s.mu.Unlock()

	links := root.UpdateStats(filtering)

	s.mu.Lock()
	s.root, s.links = root, links
	s.mu.Unlock()
}

// watch tells observe the progress until the returned function is called,
// which returns once observe will not be called any more.
func (s *Session) watch(observe Observer, roots int) (stop func()) {
	if observe == nil {
		return func() {}
	}
	quit := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				observe(s.progress(roots))
			}
		}
	}()
	return func() {
		close(quit)
		<-stopped
	}
}

func (s *Session) progress(roots int) Progress {
	s.mu.RLock()
	analyzer, phase, current := s.analyzer, s.phase, s.current
	s.mu.RUnlock()

	p := Progress{Phase: phase, Root: current, Roots: roots}
	if analyzer != nil {
		p.CurrentProgress = analyzer.GetProgress()
	}
	return p
}
