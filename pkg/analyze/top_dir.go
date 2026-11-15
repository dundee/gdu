package analyze

import (
	"io"
	"iter"
	"time"

	"github.com/dundee/gdu/v6/pkg/fs"
)

var _ treeItem = (*SimpleDir)(nil)

type SimpleFile struct {
	Name      string
	Mtime     time.Time
	Symlink   string
	Flag      rune
	Size      int64
	Usage     int64
	ItemCount int64
	IsDir     bool

	// alreadyCounted excludes duplicate hard links from totals without changing
	// the size displayed for each individual file.
	alreadyCounted bool
}

// SimpleDir is the directory the top-dir analyzer returns: only its entries
// are kept, each with the totals of its own tree.
type SimpleDir struct {
	SimpleFile
	Files    []SimpleFile
	BasePath string
	// OwnUsage is the disk usage of the blocks allocated to the directory
	// itself, excluding its entries.
	OwnUsage int64
	parent   fs.Item
}

func (d *SimpleDir) GetName() string {
	return d.Name
}

func (d *SimpleDir) GetUsage() int64 {
	return d.Usage
}

func (d *SimpleDir) GetSize() int64 {
	return d.Size
}

func (d *SimpleDir) IsDir() bool {
	return true
}

func (d *SimpleDir) GetPath() string {
	return d.BasePath + pathSep + d.Name
}

func (d *SimpleDir) GetFlag() rune               { return d.Flag }
func (d *SimpleDir) GetType() string             { return dirType }
func (d *SimpleDir) GetMtime() time.Time         { return d.Mtime }
func (d *SimpleDir) GetItemCount() int64         { return d.ItemCount }
func (d *SimpleDir) GetParent() fs.Item          { return d.parent }
func (d *SimpleDir) SetParent(parent fs.Item)    { d.parent = parent }
func (d *SimpleDir) GetMultiLinkedInode() uint64 { return 0 }

// EncodeJSON writes the directory and its entries in the format of ncdu's
// export.
func (d *SimpleDir) EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
	dir := &Dir{
		File: &File{
			Name:   d.Name,
			Mtime:  d.Mtime,
			Flag:   d.Flag,
			Size:   d.Size,
			Usage:  d.Usage,
			Parent: d.parent,
		},
		BasePath:  d.BasePath,
		ItemCount: d.ItemCount,
		OwnUsage:  d.OwnUsage,
	}
	dir.Files = d.entries(dir)
	return dir.EncodeJSON(writer, topLevel, attributes)
}

func (d *SimpleDir) itemStats(_ *fs.HardLinks, filteringFiles bool) (itemCount, size, usage int64) {
	d.updateStats(filteringFiles)
	return d.ItemCount, d.Size, d.Usage
}

// UpdateStats computes the totals of the directory from its entries. Hard
// links are counted once during the scan, so there is no index to return.
func (d *SimpleDir) UpdateStats(filteringFiles bool) *fs.HardLinks {
	d.updateStats(filteringFiles)
	return nil
}

func (d *SimpleDir) updateStats(filteringFiles bool) {
	totals := dirTotals{itemCount: 1, ownUsage: d.OwnUsage, entries: len(d.Files)}
	for _, entry := range d.Files {
		if !entry.alreadyCounted {
			totals.size += entry.Size
			totals.usage += entry.Usage
		}
		totals.itemCount += entry.ItemCount
		if !entry.IsDir {
			totals.hasFiles = true
		}

		switch entry.Flag {
		case '!', '.':
			if d.Flag != '!' {
				d.Flag = '.'
			}
		}
	}
	d.ItemCount, d.Size, d.Usage = resolveDirStats(totals, filteringFiles)
}

func (d *SimpleDir) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {
		sorted := d.entries(d)
		sortFiles(sorted, sortBy, order)

		for _, item := range sorted {
			if !yield(item) {
				return
			}
		}
	}
}

// entries returns the entries of the directory as items held by parent.
func (d *SimpleDir) entries(parent fs.Item) fs.Files {
	entries := make(fs.Files, 0, len(d.Files))
	for _, file := range d.Files {
		f := &File{
			Name:    file.Name,
			Mtime:   file.Mtime,
			Flag:    file.Flag,
			Size:    file.Size,
			Usage:   file.Usage,
			Parent:  parent,
			Symlink: file.Symlink,
		}

		if file.IsDir {
			entries = append(entries, &Dir{
				File:      f,
				ItemCount: file.ItemCount,
			})
		} else {
			entries = append(entries, f)
		}
	}
	return entries
}
