package analyze

import (
	"io"
	"iter"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

var _ fs.Item = (*SimpleDir)(nil)

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

type SimpleDir struct {
	SimpleFile
	Files    []SimpleFile
	BasePath string
	// OwnUsage is the disk usage of the blocks allocated to the directory
	// itself, excluding its entries.
	OwnUsage int64
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

func (d *SimpleDir) GetFlag() rune                                       { return d.Flag }
func (d *SimpleDir) GetType() string                                     { panic("not implemented") }
func (d *SimpleDir) GetMtime() time.Time                                 { return d.Mtime }
func (d *SimpleDir) GetItemCount() int64                                 { return d.ItemCount }
func (d *SimpleDir) GetParent() fs.Item                                  { panic("not implemented") }
func (d *SimpleDir) SetParent(parent fs.Item)                            { panic("not implemented") }
func (d *SimpleDir) GetMultiLinkedInode() uint64                         { panic("not implemented") }
func (d *SimpleDir) EncodeJSON(io.Writer, bool, fs.JSONAttributes) error { panic("not implemented") }
func (d *SimpleDir) GetItemStats(*fs.HardLinks, bool) (itemCount, size, usage int64) {
	panic("not implemented")
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

func (d *SimpleDir) AddFile(fs.Item) { panic("not implemented") }
func (d *SimpleDir) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {
		// Make a copy to avoid modifying the original slice
		sorted := make(fs.Files, 0, len(d.Files))

		for _, file := range d.Files {
			f := &File{
				Name:    file.Name,
				Mtime:   file.Mtime,
				Flag:    file.Flag,
				Size:    file.Size,
				Usage:   file.Usage,
				Parent:  d,
				Symlink: file.Symlink,
			}

			if file.IsDir {
				sorted = append(sorted, &Dir{
					File:      f,
					ItemCount: file.ItemCount,
				})
			} else {
				sorted = append(sorted, f)
			}
		}

		sortFiles(sorted, sortBy, order)

		for _, item := range sorted {
			if !yield(item) {
				return
			}
		}
	}
}
func (d *SimpleDir) RemoveFile(fs.Item)           { panic("not implemented") }
func (d *SimpleDir) RemoveFileByName(name string) { panic("not implemented") }
