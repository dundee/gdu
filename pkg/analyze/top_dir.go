package analyze

import (
	"io"
	"iter"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

var _ fs.Item = (*SimpleDir)(nil)

type TopDir struct {
	Name      string
	Size      atomic.Int64
	Usage     atomic.Int64
	ItemCount atomic.Int64
	Flag      rune
	m         sync.Mutex
}

func (d *TopDir) AddUsage(size, usage, itemCount int64) {
	d.Size.Add(size)
	d.Usage.Add(usage)
	d.ItemCount.Add(itemCount)
}

func (d *TopDir) GetUsage() (size, usage, itemCount int64) {
	return d.Size.Load(), d.Usage.Load(), d.ItemCount.Load()
}

func (d *TopDir) SetFlag(flag rune) {
	d.m.Lock()
	d.Flag = flag
	d.m.Unlock()
}

type SimpleFile struct {
	Name      string
	Flag      rune
	Size      int64
	Usage     int64
	ItemCount int64
	IsDir     bool
}

type SimpleDir struct {
	SimpleFile
	Files    []SimpleFile
	BasePath string
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

func (d *SimpleDir) GetPath() string                                  { panic("not implemented") }
func (d *SimpleDir) GetFlag() rune                                    { panic("not implemented") }
func (d *SimpleDir) IsDir() bool                                      { panic("not implemented") }
func (d *SimpleDir) GetType() string                                  { panic("not implemented") }
func (d *SimpleDir) GetMtime() time.Time                              { panic("not implemented") }
func (d *SimpleDir) GetItemCount() int64                              { panic("not implemented") }
func (d *SimpleDir) GetParent() fs.Item                               { panic("not implemented") }
func (d *SimpleDir) SetParent(parent fs.Item)                         { panic("not implemented") }
func (d *SimpleDir) GetMultiLinkedInode() uint64                      { panic("not implemented") }
func (d *SimpleDir) EncodeJSON(writer io.Writer, topLevel bool) error { panic("not implemented") }
func (d *SimpleDir) GetItemStats(linkedItems fs.HardLinkedItems) (itemCount, size, usage int64) {
	panic("not implemented")
}
func (d *SimpleDir) UpdateStats(linkedItems fs.HardLinkedItems) {
	totalSize := int64(4096)
	totalUsage := int64(4096)
	var itemCount int64
	for _, entry := range d.Files {
		totalSize += entry.Size
		totalUsage += entry.Usage
		itemCount += entry.ItemCount

		switch entry.Flag {
		case '!', '.':
			if d.Flag != '!' {
				d.Flag = '.'
			}
		}
	}
	d.ItemCount = itemCount + 1
	d.Size = totalSize
	d.Usage = totalUsage
}
func (d *SimpleDir) AddFile(fs.Item) { panic("not implemented") }
func (d *SimpleDir) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {
		// Make a copy to avoid modifying the original slice
		sorted := make(fs.Files, 0, len(d.Files))

		for _, file := range d.Files {
			f := &File{
				Name:  file.Name,
				Flag:  file.Flag,
				Size:  file.Size,
				Usage: file.Usage,
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
func (d *SimpleDir) GetFilesLocked(fs.SortBy, fs.SortOrder) iter.Seq[fs.Item] {
	panic("not implemented")
}
func (d *SimpleDir) RemoveFile(fs.Item)           { panic("not implemented") }
func (d *SimpleDir) RemoveFileByName(name string) { panic("not implemented") }
func (d *SimpleDir) RLock() func()                { panic("not implemented") }
