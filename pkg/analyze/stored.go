package analyze

import (
	"context"
	"io"
	"iter"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// StoredAnalyzer scans directories into a RecordStore, keeping in memory only
// the directories in use (--db x.badger).
type StoredAnalyzer struct {
	BaseAnalyzer
	storage records
}

// NewStoredAnalyzer returns an analyzer storing its results in store.
// Package github.com/dundee/gdu/v6/pkg/analyze/badger creates one storing
// them in a badger database.
func NewStoredAnalyzer(store RecordStore) *StoredAnalyzer {
	a := &StoredAnalyzer{storage: newRecords(store)}
	a.Init()
	return a
}

// Close closes the database, as Database says.
func (a *StoredAnalyzer) Close() error {
	return a.storage.Close()
}

// LoadAnalysis returns the analysis of path stored in the database by an
// earlier scan, as Database says.
func (a *StoredAnalyzer) LoadAnalysis(path string) (Node, error) {
	if err := checkAnalysis(a.storage, storedFormat, path); err != nil {
		return nil, err
	}
	dir, err := a.storage.dirForPath(path)
	if err != nil {
		return nil, err
	}
	return dir, nil
}

// AnalyzeDir scans path into the database, replacing its analysis, as
// Database says. The analysis is stored once UpdateStats has computed and
// stored the totals of the returned tree.
func (a *StoredAnalyzer) AnalyzeDir(ctx context.Context, path string, opts Options) Node {
	s := a.begin(ctx, opts)

	a.storage.failed.Store(false)
	if err := beginAnalysis(a.storage, storedFormat, path); err != nil {
		slog.Warn("recording the analysis", "path", path, "err", err)
		a.storage.failed.Store(true)
	}
	root := walkTree[*StoredDir](&a.BaseAnalyzer, s, &storedSink{a: a}, path, true)
	if root == nil {
		return nil
	}
	stopped := s.isCancelled()
	root.stored = func() {
		if err := endAnalysis(a.storage, storedFormat, path, stopped, a.storage.failed.Load()); err != nil {
			slog.Warn("recording the analysis", "path", path, "err", err)
		}
	}
	return root
}

// storedSink stores each directory of a scan as soon as its entries are
// known. The scanned root is returned with its entries; the directories
// below it are left to be loaded from the database when needed.
type storedSink struct {
	a    *StoredAnalyzer
	root *StoredDir
}

func (s *storedSink) openDir(parent *StoredDir, path string, root bool) (*StoredDir, bool) {
	if !root {
		parent.Dir.AddFile(newStoredDir(s.a.storage, path, parent))
	}
	dir := newStoredDir(s.a.storage, path, nil)
	dir.loaded = true
	dir.ItemCount = 1
	if root {
		s.root = dir
	}
	return dir, true
}

func (*storedSink) readDir(dir *StoredDir, path string, err error, entries int) {
	dir.Flag = getDirFlag(err, entries)
	dir.Files = make(fs.Files, 0, entries)
	dir.Mtime, dir.OwnUsage, dir.Dev = dirStats(path)
}

func (*storedSink) resolveOptions(*StoredDir) resolveOptions {
	return resolveOptions{browseArchives: true}
}

func (s *storedSink) addFile(dir *StoredDir, entry *fileEntry) int64 {
	if entry.archive != nil {
		path := filepath.Join(dir.GetPath(), entry.name)
		s.a.storeArchive(entry.archive, path)
		dir.Dir.AddFile(newStoredDir(s.a.storage, path, dir))
		return entry.archive.GetUsage()
	}
	file := entry.item(dir)
	dir.Dir.AddFile(file)
	return file.GetUsage()
}

func (*storedSink) fileError(dir *StoredDir) {
	dir.SetFlag('!')
}

func (s *storedSink) entriesDone(dir *StoredDir) {
	if err := s.a.storage.storeRecord(dir.GetPath(), dir.record()); err != nil {
		slog.Warn("storing directory", "path", dir.GetPath(), "err", err)
	}
	if dir != s.root {
		// stored, and loaded again from the database when needed
		dir.Files = nil
	}
}

func (*storedSink) closeDir(*StoredDir) {}

func (*storedSink) addDir(_, _ *StoredDir) {}

// storeArchive stores the directories of a browsed archive lying at path, so
// that they load like any other stored directory.
func (a *StoredAnalyzer) storeArchive(dir fs.Item, path string) {
	record := &storedRecord{
		Type:      dir.GetType(),
		Flag:      dir.GetFlag(),
		Size:      dir.GetSize(),
		Usage:     dir.GetUsage(),
		ItemCount: dir.GetItemCount(),
		Mtime:     dir.GetMtime(),
	}
	for entry := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		name := entry.GetName()
		// names inside an archive are whatever its author chose; joined to the
		// archive path, "..", "/" or "a/b" would name another record
		if !isPathElement(name) {
			slog.Warn("skipping archive entry", "name", name, "archive", path)
			continue
		}
		if entry.IsDir() {
			record.Entries = append(record.Entries, storedEntry{Name: name, Dir: true})
			a.storeArchive(entry, filepath.Join(path, name))
			continue
		}
		record.Entries = append(record.Entries, entryOf(entry))
	}
	if err := a.storage.storeRecord(path, record); err != nil {
		slog.Warn("storing archive directory", "path", path, "err", err)
	}
}

// StoredDir is a directory of an analysis kept in the database (--db x.badger).
//
// A StoredDir is a placeholder until it is loaded: its record, holding its
// totals and entries, is read from the database when its parent lists it, or
// when it is used directly. Loading fills the placeholder in place, so the
// same item stays the entry of its parent and the parent of its own entries.
//
// The fields inherited from Dir follow Dir's locking protocol. loadMu guards
// loading and kind, and is never taken while Dir.m is held.
type StoredDir struct {
	*Dir
	storage records
	loadMu  sync.Mutex
	loaded  bool
	// kind is the GetType of a directory inside a browsed archive
	kind string
	// stored, set on the root of a scan, records that the analysis is
	// stored once its totals are
	stored func()
}

func newStoredDir(storage records, path string, parent fs.Item) *StoredDir {
	return &StoredDir{
		Dir: &Dir{
			File:     &File{Name: filepath.Base(path), Parent: parent},
			BasePath: filepath.Dir(path),
		},
		storage: storage,
	}
}

// GetType returns the type of the directory
func (f *StoredDir) GetType() string {
	f.loadMu.Lock()
	defer f.loadMu.Unlock()
	if f.kind != "" {
		return f.kind
	}
	return f.Dir.GetType()
}

// inArchive reports whether the directory lies inside a browsed archive, or
// stands for the archive itself.
func (f *StoredDir) inArchive() bool {
	f.loadMu.Lock()
	defer f.loadMu.Unlock()
	return f.kind != ""
}

// load reads the record of the directory from the database, unless the
// directory has been loaded already or was built by a scan.
func (f *StoredDir) load() error {
	f.loadMu.Lock()
	defer f.loadMu.Unlock()
	if f.loaded {
		return nil
	}

	record, err := f.storage.loadRecord(f.GetPath())
	if err != nil {
		return err
	}

	path := f.GetPath()
	files := make(fs.Files, 0, len(record.Entries))
	for _, entry := range record.Entries {
		if entry.Dir {
			files = append(files, newStoredDir(f.storage, filepath.Join(path, entry.Name), f))
			continue
		}
		files = append(files, &File{
			Name:        entry.Name,
			Flag:        entry.Flag,
			Size:        entry.Size,
			Usage:       entry.Usage,
			SharedUsage: entry.SharedUsage,
			Mtime:       entry.Mtime,
			Mli:         entry.Mli,
			Symlink:     entry.Symlink,
			Parent:      f,
		})
	}

	f.Flag = record.Flag
	f.Size, f.Usage, f.OwnUsage, f.ItemCount = record.Size, record.Usage, record.OwnUsage, record.ItemCount
	f.Dev = record.Dev
	f.Mtime = record.Mtime
	f.Files = files
	f.kind = record.Type
	f.loaded = true
	return nil
}

// isLoaded reports whether the directory has its entries.
func (f *StoredDir) isLoaded() bool {
	f.loadMu.Lock()
	defer f.loadMu.Unlock()
	return f.loaded
}

// detached returns a loaded copy of the directory read from the database,
// which shares no items with the tree. Walking the copy instead of the
// directory leaves the tree as it is, so a walk of the whole tree does not
// keep the whole tree in memory.
func (f *StoredDir) detached() (*StoredDir, error) {
	copied := newStoredDir(f.storage, f.GetPath(), f.Parent)
	if err := copied.load(); err != nil {
		return nil, err
	}
	return copied, nil
}

// loadEntries loads the directories among files, whose totals a listing shows.
func (f *StoredDir) loadEntries(files fs.Files) {
	for _, file := range files {
		if dir, ok := file.(*StoredDir); ok {
			if err := dir.load(); err != nil {
				slog.Warn("loading directory", "path", dir.GetPath(), "err", err)
			}
		}
	}
}

// record returns the record of the directory.
func (f *StoredDir) record() *storedRecord {
	record := &storedRecord{
		Type:      f.kind,
		Flag:      f.Flag,
		Size:      f.Size,
		Usage:     f.Usage,
		OwnUsage:  f.OwnUsage,
		ItemCount: f.ItemCount,
		Dev:       f.Dev,
		Mtime:     f.Mtime,
		Entries:   make([]storedEntry, 0, len(f.Files)),
	}
	for _, file := range f.Files {
		if file.IsDir() {
			record.Entries = append(record.Entries, storedEntry{Name: file.GetName(), Dir: true})
			continue
		}
		record.Entries = append(record.Entries, entryOf(file))
	}
	return record
}

// entryOf returns the stored entry of a file.
func entryOf(file fs.Item) storedEntry {
	entry := storedEntry{
		Name:  file.GetName(),
		Flag:  file.GetFlag(),
		Size:  file.GetSize(),
		Usage: file.GetUsage(),
		Mtime: file.GetMtime(),
		Mli:   file.GetMultiLinkedInode(),
	}
	if f, ok := file.(*File); ok {
		entry.SharedUsage = f.SharedUsage
	}
	if symlink, ok := file.(fs.SymlinkItem); ok {
		entry.Symlink = symlink.GetSymlinkTarget()
	}
	return entry
}

// store stores the directory.
func (f *StoredDir) store() {
	if err := f.storage.storeRecord(f.GetPath(), f.record()); err != nil {
		slog.Warn("storing directory", "path", f.GetPath(), "err", err)
	}
}

// GetFiles returns files in directory as a sorted iterator, loading the
// directories among them. Like Dir.GetFiles, it leaves locking to the caller.
func (f *StoredDir) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	f.loadEntries(f.Files)
	return f.Dir.GetFiles(sortBy, order)
}

// EncodeJSON writes JSON representation of the directory and everything in
// it, reading from the database the directories nobody has loaded.
func (f *StoredDir) EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
	if f.isLoaded() {
		return f.Dir.EncodeJSON(writer, topLevel, attributes)
	}
	copied, err := f.detached()
	if err != nil {
		return err
	}
	return copied.Dir.EncodeJSON(writer, topLevel, attributes)
}

// copyFiles returns a copy of files so callers cannot mutate the slice.
func copyFiles(files fs.Files) fs.Files {
	result := make(fs.Files, len(files))
	copy(result, files)
	return result
}

// AddFile adds item to the directory and stores the directory.
func (f *StoredDir) AddFile(item fs.Item) {
	f.Files = append(f.Files, item)
	f.store()
}

// RemoveFile removes file from stored directory
// It also updates size and item count of parent directories
func (f *StoredDir) RemoveFile(item fs.Item) {
	f.Files = f.Files.Remove(item)

	removeFromTotals(f, item)
}

// adjustStats adds the deltas to the totals of the directory and stores it.
func (f *StoredDir) adjustStats(itemCount, size, usage int64) {
	f.ItemCount += itemCount
	f.Size += size
	f.Usage += usage
	f.store()
}

// itemStats returns item count, apparent usage and real usage of this dir
func (f *StoredDir) itemStats(links *fs.HardLinks, filteringFiles bool) (itemCount, size, usage int64) {
	f.updateStats(links, filteringFiles)
	return f.GetItemCount(), f.GetSize(), f.GetUsage()
}

// UpdateStats computes the totals of the tree topped by the directory, which
// keeps the index of its hard-linked files, and returns the index.
func (f *StoredDir) UpdateStats(filteringFiles bool) *fs.HardLinks {
	links := fs.NewHardLinks()
	f.updateStats(links, filteringFiles)
	f.links = links
	return links
}

// updateStats computes the totals of the directory from its entries and
// stores them. For a directory nobody has loaded, they are computed on a
// detached copy and only the totals are kept, so computing the totals of a
// tree does not keep the whole tree in memory.
func (f *StoredDir) updateStats(links *fs.HardLinks, filteringFiles bool) {
	if !f.isLoaded() {
		copied, err := f.detached()
		if err != nil {
			slog.Warn("loading directory", "path", f.GetPath(), "err", err)
			return
		}
		copied.updateStats(links, filteringFiles)

		mtime, flag := copied.Mtime, copied.Flag
		itemCount, size, usage, ownUsage := copied.ItemCount, copied.Size, copied.Usage, copied.OwnUsage

		f.Mtime, f.Flag = mtime, flag
		f.ItemCount, f.Size, f.Usage, f.OwnUsage = itemCount, size, usage, ownUsage
		return
	}

	if f.inArchive() {
		// what a browsed archive holds does not change: its totals are
		// those stored by the scan (see finishArchive)
		return
	}

	files := copyFiles(f.Files)
	curMtime := f.Mtime
	curFlag := f.Flag
	dev := f.Dev

	totals, mtime, flag := aggregateDirEntries(files, curMtime, curFlag, dev, links, filteringFiles)

	f.Mtime = mtime
	f.Flag = flag
	totals.ownUsage = f.OwnUsage
	f.ItemCount, f.Size, f.Usage = resolveDirStats(totals, filteringFiles)
	f.store()

	if f.stored != nil {
		f.stored()
		f.stored = nil
	}
}
