package analyze

import (
	"iter"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// fileFlagMu protects the mutable Flag field on published File values. File
// keeps its fields public for compatibility, so hard-link accounting can
// update Flag after a file has been exposed to preview readers.
var fileFlagMu sync.RWMutex

// File struct
type File struct {
	Mtime  time.Time
	Parent fs.Item
	Name   string
	Size   int64
	Usage  int64
	Mli    uint64
	Flag   rune
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
	fileFlagMu.RLock()
	defer fileFlagMu.RUnlock()
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
	fileFlagMu.RLock()
	defer fileFlagMu.RUnlock()
	if f.Flag == '@' {
		return "Other"
	}
	return "File"
}

// GetItemCount returns 1 for file
func (f *File) GetItemCount() int64 {
	return 1
}

// GetMultiLinkedInode returns inode number of multilinked file
func (f *File) GetMultiLinkedInode() uint64 {
	return f.Mli
}

func (f *File) alreadyCounted(linkedItems fs.HardLinkedItems) bool {
	mli := f.Mli
	counted := false
	if mli > 0 {
		fileFlagMu.Lock()
		f.Flag = 'H'
		fileFlagMu.Unlock()
		if _, ok := linkedItems[mli]; ok {
			counted = true
		}
		linkedItems[mli] = append(linkedItems[mli], f)
	}
	return counted
}

// CreateFileItem creates a File from an os.FileInfo with correct platform-specific attributes
func CreateFileItem(name string, info os.FileInfo) *File {
	file := &File{
		Name: name,
		Size: info.Size(),
		Flag: getFlag(info),
	}
	setPlatformSpecificAttrs(file, info)
	return file
}

// GetItemStats returns 1 as count of items, apparent usage and real usage of this file
func (f *File) GetItemStats(linkedItems fs.HardLinkedItems, filteringFiles bool) (itemCount, size, usage int64) {
	if f.alreadyCounted(linkedItems) {
		return 1, 0, 0
	}
	return 1, f.GetSize(), f.GetUsage()
}

// UpdateStats does nothing on file
func (f *File) UpdateStats(linkedItems fs.HardLinkedItems)                  {}
func (f *File) UpdateStatsWithFileFiltering(linkedItems fs.HardLinkedItems) {}

// GetFiles returns all files in directory
func (f *File) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {}
}

// GetFilesLocked returns all files in directory
func (f *File) GetFilesLocked(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return f.GetFiles(sortBy, order)
}

// RLock panics on file
func (f *File) RLock() func() {
	panic("RLock should not be called on file")
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

// Dir struct
type Dir struct {
	*File
	BasePath  string
	Files     fs.Files
	ItemCount int64
	m         sync.RWMutex
}

func snapshotDir(source *Dir, parent fs.Item) *Dir {
	source.m.RLock()
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
	}
	children := append(fs.Files(nil), source.Files...)
	source.m.RUnlock()

	for _, child := range children {
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
		for child := range source.GetFilesLocked(fs.SortByName, fs.SortAsc) {
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
	return &File{
		Mtime:  source.GetMtime(),
		Parent: parent,
		Name:   source.GetName(),
		Size:   source.GetSize(),
		Usage:  source.GetUsage(),
		Mli:    source.GetMultiLinkedInode(),
		Flag:   source.GetFlag(),
	}
}

// AddFile add item to files
func (f *Dir) AddFile(item fs.Item) {
	f.m.Lock()
	defer f.m.Unlock()
	f.Files = append(f.Files, item)
}

// SetFlag updates the directory flag while preserving snapshot consistency.
func (f *Dir) SetFlag(flag rune) {
	f.m.Lock()
	f.Flag = flag
	f.m.Unlock()
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

// GetFilesLocked returns all files in directory as a sorted iterator
// It is safe to call this function from multiple goroutines
func (f *Dir) GetFilesLocked(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {
		f.m.RLock()
		defer f.m.RUnlock()

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

// GetFlag returns the current directory flag.
func (f *Dir) GetFlag() rune {
	f.m.RLock()
	defer f.m.RUnlock()
	return f.Flag
}

// GetSize returns the current apparent size.
func (f *Dir) GetSize() int64 {
	f.m.RLock()
	defer f.m.RUnlock()
	return f.Size
}

// GetUsage returns the current disk usage.
func (f *Dir) GetUsage() int64 {
	f.m.RLock()
	defer f.m.RUnlock()
	return f.Usage
}

// GetMtime returns the current modification time.
func (f *Dir) GetMtime() time.Time {
	f.m.RLock()
	defer f.m.RUnlock()
	return f.Mtime
}

// GetItemCount returns number of files in dir
func (f *Dir) GetItemCount() int64 {
	f.m.RLock()
	defer f.m.RUnlock()
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
func (f *Dir) GetItemStats(linkedItems fs.HardLinkedItems, filteringFiles bool) (itemCount, size, usage int64) {
	f.updateStats(linkedItems, filteringFiles)
	return f.GetItemCount(), f.GetSize(), f.GetUsage()
}

func (f *Dir) UpdateStats(linkedItems fs.HardLinkedItems) {
	f.updateStats(linkedItems, false)
}

func (f *Dir) UpdateStatsWithFileFiltering(linkedItems fs.HardLinkedItems) {
	f.updateStats(linkedItems, true)
}

// UpdateStats recursively updates size and item count
func (f *Dir) updateStats(linkedItems fs.HardLinkedItems, filteringFiles bool) {
	// Snapshot the file list under the read lock so it is safe to compute stats
	// even while the analyzer is still appending items in another goroutine
	// (e.g. when previewing a directory mid-scan).
	f.m.RLock()
	files := make(fs.Files, len(f.Files))
	copy(files, f.Files)
	mtime := f.Mtime
	flag := f.Flag
	f.m.RUnlock()

	totalSize := int64(0)
	totalUsage := int64(0)
	var itemCount int64 = 1
	var hasFiles bool
	for _, entry := range files {
		count, size, usage := entry.GetItemStats(linkedItems, filteringFiles)
		totalSize += size
		totalUsage += usage
		itemCount += count

		entryMtime := entry.GetMtime()
		if entryMtime.After(mtime) {
			mtime = entryMtime
		}

		if !entry.IsDir() {
			hasFiles = true
		}

		switch entry.GetFlag() {
		case '!', '.':
			if flag != '!' {
				flag = '.'
			}
		}
	}

	f.m.Lock()
	defer f.m.Unlock()
	f.Mtime = mtime
	f.Flag = flag

	// no files, or just empty dirs
	if len(files) == 0 || (!hasFiles && filteringFiles && itemCount == int64(len(files)+1)) {
		f.ItemCount = 1
		f.Size = totalSize + 512
		f.Usage = 0
	} else {
		f.ItemCount = itemCount
		f.Size = totalSize
		f.Usage = totalUsage
	}
}

// RemoveFile removes item from dir, updates size and item count
func (f *Dir) RemoveFile(item fs.Item) {
	f.m.Lock()
	defer f.m.Unlock()

	f.Files = f.Files.Remove(item)

	cur := f
	for {
		cur.ItemCount -= item.GetItemCount()
		cur.Size -= item.GetSize()
		cur.Usage -= item.GetUsage()

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent.(*Dir)
	}
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

// RLock read locks dir
func (f *Dir) RLock() func() {
	f.m.RLock()
	return f.m.RUnlock
}

// RemoveFileByName removes item by name from dir
func (f *Dir) RemoveFileByName(name string) {
	f.m.Lock()
	defer f.m.Unlock()

	idx, ok := f.Files.FindByName(name)
	if !ok {
		return
	}
	item := f.Files[idx]
	f.Files = append(f.Files[:idx], f.Files[idx+1:]...)

	cur := f
	for {
		cur.ItemCount -= item.GetItemCount()
		cur.Size -= item.GetSize()
		cur.Usage -= item.GetUsage()

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent.(*Dir)
	}
}
