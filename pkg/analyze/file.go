package analyze

import (
	"iter"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

var _ fs.SymlinkItem = (*File)(nil)

// File struct
type File struct {
	Mtime   time.Time
	Parent  fs.Item
	Name    string
	Symlink string
	Size    int64
	Usage   int64
	// SharedUsage is the bytes shared with other files, measured only with
	// --exclusive-size; Usage then excludes them.
	SharedUsage int64
	Mli         uint64
	Flag        rune
}

// GetName returns name of dir
func (f *File) GetName() string {
	return f.Name
}

// IsDir returns false for file
func (f *File) IsDir() bool {
	return false
}

// GetParent returns parent dir
func (f *File) GetParent() fs.Item {
	return f.Parent
}

// SetParent sets parent dir
func (f *File) SetParent(parent fs.Item) {
	f.Parent = parent
}

// GetPath returns absolute Get of the file
func (f *File) GetPath() string {
	return filepath.Join(f.Parent.GetPath(), f.Name)
}

// GetFlag returns flag of the file
func (f *File) GetFlag() rune {
	return f.Flag
}

// GetSize returns size of the file
func (f *File) GetSize() int64 {
	return f.Size
}

// GetUsage returns usage of the file
func (f *File) GetUsage() int64 {
	return f.Usage
}

// GetMtime returns mtime of the file
func (f *File) GetMtime() time.Time {
	return f.Mtime
}

// GetType returns name type of item
func (f *File) GetType() string {
	if f.Flag == '@' {
		if f.Symlink != "" {
			return "Symlink"
		}
		return "Other"
	}
	return "File"
}

// GetSymlinkTarget returns the target of a symlink, or empty string
func (f *File) GetSymlinkTarget() string {
	return f.Symlink
}

// GetItemCount returns 1 for file
func (f *File) GetItemCount() int64 {
	return 1
}

// GetMultiLinkedInode returns inode number of multilinked file
func (f *File) GetMultiLinkedInode() uint64 {
	return f.Mli
}

// CreateFileItem creates a File from an os.FileInfo with correct platform-specific attributes
func CreateFileItem(path string, info os.FileInfo) *File {
	usage, link := fileStats(info, path)
	return &File{
		Name:  filepath.Base(path),
		Size:  info.Size(),
		Usage: usage,
		Mtime: info.ModTime(),
		Mli:   link.ino,
		Flag:  getFlag(info),
	}
}

// GetItemStats returns 1 as count of items, apparent usage and real usage of
// this file. A file with several hard links is counted by its directory,
// which knows its device.
func (f *File) GetItemStats(*fs.HardLinks, bool) (itemCount, size, usage int64) {
	return 1, f.GetSize(), f.GetUsage()
}

// UpdateStats does nothing on file
func (f *File) UpdateStats(bool) *fs.HardLinks { return nil }

// GetFiles returns all files in directory
func (f *File) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {}
}

// AddFile panics on file
func (f *File) AddFile(item fs.Item) {
	panic("AddFile should not be called on file")
}

// RemoveFile panics on file
func (f *File) RemoveFile(item fs.Item) {
	panic("RemoveFile should not be called on file")
}

// RemoveFileByName panics on file
func (f *File) RemoveFileByName(name string) {
	panic("RemoveFileByName should not be called on file")
}

// EmptyDirSize is the apparent size gdu assigns to a directory that has no
// counted children. It is a fixed sentinel (not the real filesystem block
// size) written by updateStats, SimpleDir.UpdateStats and the parallel top-dir
// analyzer, and recognised again on import. Keep every producer and consumer
// of this value referencing this constant so they can never drift apart. It is
// an apparent size only; it never affects disk usage.
const EmptyDirSize int64 = 512

// dirTotals is the aggregate of a directory's counted children, before gdu's
// empty-directory rule is applied.
//
// itemCount includes the directory itself, so it starts at 1. entries is the
// number of children that were counted, and hasFiles records whether any of
// them was not itself a directory. ownUsage is the disk usage of the directory
// itself (see Dir.OwnUsage), which is read from the filesystem, never assumed:
// a small directory on XFS, for example, occupies no block of its own.
type dirTotals struct {
	size      int64
	usage     int64
	itemCount int64
	ownUsage  int64
	entries   int
	hasFiles  bool
}

// resolveDirStats applies gdu's empty-directory rule to a directory's
// aggregated child totals.
//
// A directory that counted no children — or, when file filtering is active,
// one whose entries are all themselves empty directories — reports EmptyDirSize
// as its apparent size and collapses to a single item. Its disk usage is never
// collapsed: every directory is charged the blocks it really occupies, as du
// does, so a tree of empty directories costs what those directories allocate.
//
// This is the single definition of that rule. Every analyzer must route through
// it, otherwise the numbers gdu reports depend on which analyzer produced them.
func resolveDirStats(totals dirTotals, filteringFiles bool) (itemCount, size, usage int64) {
	onlyEmptyDirs := !totals.hasFiles && filteringFiles &&
		totals.itemCount == int64(totals.entries+1)

	usage = totals.usage + totals.ownUsage
	if totals.entries == 0 || onlyEmptyDirs {
		return 1, totals.size + EmptyDirSize, usage
	}
	return totals.itemCount, totals.size, usage
}

// aggregateDirEntries folds the stats of entries into the totals their parent
// directory should report, and returns the parent's mtime and flag updated from
// its children. mtime and flag are seeded with the parent's current values.
//
// A file with several hard links is counted by the first of its links added
// to links; its entries lie on the device dev.
func aggregateDirEntries(
	entries fs.Files,
	mtime time.Time,
	flag rune,
	dev uint64,
	links *fs.HardLinks,
	filteringFiles bool,
) (totals dirTotals, newMtime time.Time, newFlag rune) {
	totals = dirTotals{itemCount: 1, entries: len(entries)}

	for _, entry := range entries {
		count, size, usage := entry.GetItemStats(links, filteringFiles)
		if ino := entry.GetMultiLinkedInode(); ino != 0 && !entry.IsDir() &&
			!links.Add(fs.InodeKey{Dev: dev, Ino: ino}, entry) {
			size, usage = 0, 0
		}
		totals.size += size
		totals.usage += usage
		totals.itemCount += count

		if entryMtime := entry.GetMtime(); entryMtime.After(mtime) {
			mtime = entryMtime
		}

		if !entry.IsDir() {
			totals.hasFiles = true
		}

		switch entry.GetFlag() {
		case '!', '.':
			if flag != '!' {
				flag = '.'
			}
		}
	}

	return totals, mtime, flag
}

// Dir struct
type Dir struct {
	*File
	BasePath  string
	Files     fs.Files
	ItemCount int64
	// OwnUsage is the disk usage of the blocks allocated to the directory
	// itself, excluding its entries.
	OwnUsage int64
	// Dev is the device the directory and its files lie on, which tells
	// hard-linked files on different filesystems apart.
	Dev           uint64
	statsFromJSON bool
	// links is the index of the hard-linked files of the tree, kept by the
	// directory topping the tree when its totals are computed
	links *fs.HardLinks
}

// snapshotDir returns a copy of the tree topped by source, which no one may
// change meanwhile.
func snapshotDir(source *Dir, parent fs.Item) *Dir {
	snapshot := &Dir{
		File: &File{
			Mtime:  source.Mtime,
			Parent: parent,
			Name:   source.Name,
			Size:   source.Size,
			Usage:  source.Usage,
			Mli:    source.Mli,
			Flag:   source.Flag,
		},
		BasePath:  source.BasePath,
		Files:     make(fs.Files, 0, len(source.Files)),
		ItemCount: source.ItemCount,
		OwnUsage:  source.OwnUsage,
		Dev:       source.Dev,
	}
	for _, child := range source.Files {
		snapshot.Files = append(snapshot.Files, snapshotItem(child, snapshot))
	}
	return snapshot
}

func snapshotItem(source, parent fs.Item) fs.Item {
	switch item := source.(type) {
	case *ZipDir:
		snapshot := &ZipDir{Dir: snapshotDir(item.Dir, parent), zipPath: item.zipPath}
		reparentSnapshotChildren(snapshot.Dir, snapshot)
		return snapshot
	case *TarDir:
		snapshot := &TarDir{Dir: snapshotDir(item.Dir, parent), tarPath: item.tarPath}
		reparentSnapshotChildren(snapshot.Dir, snapshot)
		return snapshot
	case *ZipFile:
		return &ZipFile{
			File:      snapshotFile(item.File, parent),
			zipPath:   item.zipPath,
			inZipPath: item.inZipPath,
		}
	case *TarFile:
		return &TarFile{
			File:      snapshotFile(item.File, parent),
			tarPath:   item.tarPath,
			inTarPath: item.inTarPath,
		}
	case *Dir:
		return snapshotDir(item, parent)
	case *File:
		return snapshotFile(item, parent)
	}

	if source.IsDir() {
		snapshot := &Dir{
			File:      snapshotFile(source, parent),
			Files:     make(fs.Files, 0),
			ItemCount: source.GetItemCount(),
		}
		for child := range source.GetFiles(fs.SortByName, fs.SortAsc) {
			snapshot.Files = append(snapshot.Files, snapshotItem(child, snapshot))
		}
		return snapshot
	}
	return snapshotFile(source, parent)
}

func reparentSnapshotChildren(dir *Dir, parent fs.Item) {
	for _, child := range dir.Files {
		child.SetParent(parent)
	}
}

func snapshotFile(source, parent fs.Item) *File {
	snapshot := &File{
		Mtime:  source.GetMtime(),
		Parent: parent,
		Name:   source.GetName(),
		Size:   source.GetSize(),
		Usage:  source.GetUsage(),
		Mli:    source.GetMultiLinkedInode(),
		Flag:   source.GetFlag(),
	}
	if file, ok := source.(*File); ok {
		snapshot.SharedUsage = file.SharedUsage
	}
	return snapshot
}

// AddFile add item to files
func (f *Dir) AddFile(item fs.Item) {
	f.Files = append(f.Files, item)
}

// SetFlag updates the directory flag while preserving snapshot consistency.
func (f *Dir) SetFlag(flag rune) {
	f.Flag = flag
}

// SetStatsFromJSON marks directory statistics decoded from an export as authoritative.
func (f *Dir) SetStatsFromJSON() {
	f.statsFromJSON = true
}

// HasEmptyDirPlaceholderStats reports whether this directory's stats are
// indistinguishable from the placeholder updateStats produces for a directory
// with no counted children (see EmptyDirSize). Such stats carry no information
// worth pinning from an import: recomputation reproduces exactly the same
// values, so callers should let updateStats run rather than preserving them.
//
// The items == 1 check is load-bearing: a depth-truncated directory that held
// real content always exports with ItemCount >= 2, so it can never be mistaken
// for a placeholder and its stats are always preserved.
func (f *Dir) HasEmptyDirPlaceholderStats() bool {
	return f.Size == EmptyDirSize && f.Usage == f.OwnUsage && f.ItemCount == 1
}

// GetFiles returns all files in directory as a sorted iterator
func (f *Dir) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {
		// Make a copy to avoid modifying the original slice
		sorted := make(fs.Files, len(f.Files))
		copy(sorted, f.Files)
		sortFiles(sorted, sortBy, order)

		for _, item := range sorted {
			if !yield(item) {
				return
			}
		}
	}
}

// GetType returns name type of item
func (f *Dir) GetType() string {
	return "Directory"
}

// GetItemCount returns number of files in dir
func (f *Dir) GetItemCount() int64 {
	return f.ItemCount
}

// IsDir returns true for dir
func (f *Dir) IsDir() bool {
	return true
}

// GetPath returns absolute path of the file
func (f *Dir) GetPath() string {
	if f.BasePath != "" {
		return filepath.Join(f.BasePath, f.Name)
	}
	if f.Parent != nil {
		return filepath.Join(f.Parent.GetPath(), f.Name)
	}
	return f.Name
}

// GetItemStats returns item count, apparent usage and real usage of this dir
func (f *Dir) GetItemStats(links *fs.HardLinks, filteringFiles bool) (itemCount, size, usage int64) {
	f.updateStats(links, filteringFiles)
	return f.GetItemCount(), f.GetSize(), f.GetUsage()
}

// UpdateStats computes the totals of the tree topped by the directory, which
// keeps the index of its hard-linked files, and returns the index.
func (f *Dir) UpdateStats(filteringFiles bool) *fs.HardLinks {
	links := fs.NewHardLinks()
	f.updateStats(links, filteringFiles)
	f.links = links
	return links
}

// updateStats recursively updates size and item count
func (f *Dir) updateStats(links *fs.HardLinks, filteringFiles bool) {
	if f.statsFromJSON {
		return
	}

	totals, mtime, flag := aggregateDirEntries(f.Files, f.Mtime, f.Flag, f.Dev, links, filteringFiles)

	f.Mtime = mtime
	f.Flag = flag
	totals.ownUsage = f.OwnUsage
	f.ItemCount, f.Size, f.Usage = resolveDirStats(totals, filteringFiles)
}

// RemoveFile removes item from dir, updates size and item count
func (f *Dir) RemoveFile(item fs.Item) {
	f.Files = f.Files.Remove(item)

	removeFromTotals(f, item)
}

// adjustStats adds the deltas to the totals of the directory alone. It is
// updated under its own lock, so readers calling GetSize, GetUsage or
// GetItemCount see consistent totals.
func (f *Dir) adjustStats(itemCount, size, usage int64) {
	f.ItemCount += itemCount
	f.Size += size
	f.Usage += usage
}

// hardLinks returns the index of the hard-linked files kept by the directory,
// if it topped a tree whose totals were computed.
func (f *Dir) hardLinks() *fs.HardLinks {
	return f.links
}

// device returns the device the directory lies on.
func (f *Dir) device() uint64 {
	return f.Dev
}

// sortFiles sorts files in place according to sortBy and order
func sortFiles(files fs.Files, sortBy fs.SortBy, order fs.SortOrder) {
	var sorter sort.Interface
	switch sortBy {
	case fs.SortByName:
		sorter = fs.ByName(files)
	case fs.SortByItemCount:
		sorter = fs.ByItemCount(files)
	case fs.SortByMtime:
		sorter = fs.ByMtime(files)
	case fs.SortByApparentSize:
		sorter = fs.ByApparentSize(files)
	case fs.SortBySize:
		sorter = files
	}

	if order == fs.SortDesc {
		sort.Sort(sort.Reverse(sorter))
	} else {
		sort.Sort(sorter)
	}
}

// RemoveFileByName removes item by name from dir
func (f *Dir) RemoveFileByName(name string) {
	idx, ok := f.Files.FindByName(name)
	if !ok {
		return
	}
	item := f.Files[idx]
	f.Files = append(f.Files[:idx], f.Files[idx+1:]...)

	removeFromTotals(f, item)
}
