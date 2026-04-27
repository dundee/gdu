package analyze

import (
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestTopDirAddUsage(t *testing.T) {
	td := &TopDir{Name: "test"}
	td.AddUsage(100, 200, 5)
	assert.Equal(t, int64(100), td.Size.Load())
	assert.Equal(t, int64(200), td.Usage.Load())
	assert.Equal(t, int64(5), td.ItemCount.Load())

	td.AddUsage(50, 80, 3)
	assert.Equal(t, int64(150), td.Size.Load())
	assert.Equal(t, int64(280), td.Usage.Load())
	assert.Equal(t, int64(8), td.ItemCount.Load())
}

func TestTopDirSetFlag(t *testing.T) {
	td := &TopDir{Name: "test", Flag: ' '}
	td.SetFlag('!')
	assert.Equal(t, '!', td.Flag)
}

func TestSimpleDirGetName(t *testing.T) {
	d := &SimpleDir{SimpleFile: SimpleFile{Name: "mydir"}}
	assert.Equal(t, "mydir", d.GetName())
}

func TestSimpleDirGetSize(t *testing.T) {
	d := &SimpleDir{SimpleFile: SimpleFile{Size: 1234}}
	assert.Equal(t, int64(1234), d.GetSize())
}

func TestSimpleDirGetUsage(t *testing.T) {
	d := &SimpleDir{SimpleFile: SimpleFile{Usage: 5678}}
	assert.Equal(t, int64(5678), d.GetUsage())
}

func TestSimpleDirUpdateStats(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true},
		Files: []SimpleFile{
			{Name: "file1", Size: 100, Usage: 200, ItemCount: 1},
			{Name: "file2", Size: 50, Usage: 80, ItemCount: 1},
			{Name: "subdir", Size: 300, Usage: 400, ItemCount: 5, IsDir: true},
		},
	}

	d.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, int64(100+50+300+4096), d.Size)
	assert.Equal(t, int64(200+80+400+4096), d.Usage)
	assert.Equal(t, int64(1+1+5+1), d.ItemCount)
}

func TestSimpleDirUpdateStatsErrorFlag(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true},
		Files: []SimpleFile{
			{Name: "sub", Size: 100, Usage: 200, ItemCount: 1, Flag: '!'},
		},
	}

	d.UpdateStats(make(fs.HardLinkedItems))
	assert.Equal(t, '.', d.Flag)
}

func TestSimpleDirUpdateStatsErrorFlagPreserved(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true, Flag: '!'},
		Files: []SimpleFile{
			{Name: "sub", Size: 100, Usage: 200, ItemCount: 1, Flag: '!'},
		},
	}

	d.UpdateStats(make(fs.HardLinkedItems))
	// '!' flag on dir should be preserved (not downgraded to '.')
	assert.Equal(t, '!', d.Flag)
}

func TestSimpleDirUpdateStatsDotFlag(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true},
		Files: []SimpleFile{
			{Name: "sub", Size: 100, Usage: 200, ItemCount: 1, Flag: '.'},
		},
	}

	d.UpdateStats(make(fs.HardLinkedItems))
	assert.Equal(t, '.', d.Flag)
}

func TestSimpleDirGetFilesSort(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true},
		Files: []SimpleFile{
			{Name: "small", Size: 10, Usage: 10},
			{Name: "big", Size: 100, Usage: 100},
			{Name: "medium", Size: 50, Usage: 50},
		},
	}

	// Collect files sorted by size descending (default sort is desc for size)
	var names []string
	for item := range d.GetFiles(fs.SortBySize, fs.SortAsc) {
		names = append(names, item.GetName())
	}
	assert.Equal(t, 3, len(names))
}

func TestSimpleDirGetFilesDirVsFile(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true},
		Files: []SimpleFile{
			{Name: "afile", Size: 10, Usage: 10, IsDir: false},
			{Name: "adir", Size: 100, Usage: 100, IsDir: true, ItemCount: 5},
		},
	}

	var items []fs.Item
	for item := range d.GetFiles(fs.SortBySize, fs.SortAsc) {
		items = append(items, item)
	}
	assert.Equal(t, 2, len(items))

	// The dir entry should be a *Dir with ItemCount
	for _, item := range items {
		if item.GetName() == "adir" {
			assert.True(t, item.IsDir())
			assert.Equal(t, int64(5), item.GetItemCount())
		}
	}
}

func TestSimpleDirPanics(t *testing.T) {
	d := &SimpleDir{}

	assert.Panics(t, func() { d.GetPath() })
	assert.Panics(t, func() { d.GetFlag() })
	assert.Panics(t, func() { d.IsDir() })
	assert.Panics(t, func() { d.GetType() })
	assert.Panics(t, func() { d.GetMtime() })
	assert.Panics(t, func() { d.GetItemCount() })
	assert.Panics(t, func() { d.GetParent() })
	assert.Panics(t, func() { d.SetParent(nil) })
	assert.Panics(t, func() { d.GetMultiLinkedInode() })
	assert.Panics(t, func() { d.EncodeJSON(nil, false) })
	assert.Panics(t, func() { d.GetItemStats(nil) })
	assert.Panics(t, func() { d.AddFile(nil) })
	assert.Panics(t, func() {
		d.GetFilesLocked(fs.SortBySize, fs.SortAsc)
	})
	assert.Panics(t, func() { d.RemoveFile(nil) })
	assert.Panics(t, func() { d.RemoveFileByName("x") })
	assert.Panics(t, func() { d.RLock() })
}
