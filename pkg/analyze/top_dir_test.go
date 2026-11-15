package analyze

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	d.UpdateStats(false)

	assert.Equal(t, int64(100+50+300), d.Size)
	assert.Equal(t, int64(200+80+400), d.Usage)
	assert.Equal(t, int64(1+1+5+1), d.ItemCount)
}

func TestEmptySimpleDirUpdateStats(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true},
		Files:      []SimpleFile{},
	}

	d.UpdateStats(false)

	assert.Equal(t, int64(512), d.Size)
	assert.Equal(t, int64(0), d.Usage)
	assert.Equal(t, int64(1), d.ItemCount, "the directory itself, as Dir counts it")
}

func TestSimpleDirUpdateStatsErrorFlag(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true},
		Files: []SimpleFile{
			{Name: "sub", Size: 100, Usage: 200, ItemCount: 1, Flag: '!'},
		},
	}

	d.UpdateStats(false)
	assert.Equal(t, '.', d.Flag)
}

func TestSimpleDirUpdateStatsErrorFlagPreserved(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "root", IsDir: true, Flag: '!'},
		Files: []SimpleFile{
			{Name: "sub", Size: 100, Usage: 200, ItemCount: 1, Flag: '!'},
		},
	}

	d.UpdateStats(false)
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

	d.UpdateStats(false)
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
		BasePath: ".",
	}

	assert.True(t, d.IsDir())
	assert.Equal(t, "root", d.GetName())
	assert.Equal(t, "./root", d.GetPath())

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

// The top-dir analyzer's directory is an item like any other: it reads, takes
// a parent and exports, rather than panicking.
func TestSimpleDirIsAnItem(t *testing.T) {
	d := &SimpleDir{
		SimpleFile: SimpleFile{Name: "top", Flag: ' '},
		BasePath:   "/base",
		Files: []SimpleFile{
			{Name: "sub", IsDir: true, Size: 10, Usage: 4096, ItemCount: 3, Flag: ' '},
			{Name: "file", Size: 5, Usage: 4096, ItemCount: 1, Flag: ' '},
		},
	}
	d.UpdateStats(false)

	assert.Equal(t, "Directory", d.GetType())
	assert.Nil(t, d.GetParent())
	assert.Equal(t, uint64(0), d.GetMultiLinkedInode())
	parent := &Dir{File: &File{Name: "parent"}}
	d.SetParent(parent)
	assert.Same(t, parent, d.GetParent())

	count, size, usage := d.itemStats(nil, false)
	assert.Equal(t, int64(5), count)
	assert.Equal(t, int64(15), size)
	assert.Equal(t, int64(8192), usage)

	var buff bytes.Buffer
	require.NoError(t, d.EncodeJSON(&buff, true, nil))
	assert.Contains(t, buff.String(), `"name":"/base/top"`)
	assert.Contains(t, buff.String(), `"name":"sub"`)
	assert.Contains(t, buff.String(), `"name":"file","asize":5`)
}
