package analyze

import (
	"testing"

	"github.com/dundee/gdu/v6/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nodeTree struct {
	top, sub      *Dir
	file, subFile *File
}

// nodeTestTree returns a directory holding a 100-byte file and a directory
// holding a 50-byte file, with its totals computed.
func nodeTestTree() nodeTree {
	top := &Dir{File: &File{Name: "top"}, BasePath: "/"}
	sub := &Dir{File: &File{Name: "sub", Parent: top}}
	file := &File{Name: "file", Size: 100, Usage: 4096, Parent: top}
	subFile := &File{Name: "file", Size: 50, Usage: 4096, Parent: sub}
	sub.Files = fs.Files{subFile}
	top.Files = fs.Files{sub, file}
	top.UpdateStats(false)
	return nodeTree{top: top, sub: sub, file: file, subFile: subFile}
}

func TestRemoveEntry(t *testing.T) {
	tree := nodeTestTree()
	top, sub := tree.top, tree.sub
	count, size, usage := top.GetItemCount(), top.GetSize(), top.GetUsage()

	require.NoError(t, RemoveEntry(sub, tree.subFile))

	assert.Empty(t, sub.Files)
	assert.Equal(t, count-1, top.GetItemCount())
	assert.Equal(t, size-50, top.GetSize())
	assert.Equal(t, usage-4096, top.GetUsage())
}

// Replacing a file by an emptied one keeps it counted, with its new size.
func TestReplaceEntry(t *testing.T) {
	tree := nodeTestTree()
	top, file := tree.top, tree.file
	count, size, usage := top.GetItemCount(), top.GetSize(), top.GetUsage()
	emptied := &File{Name: "file"}

	require.NoError(t, ReplaceEntry(top, file, emptied))

	assert.Same(t, top, emptied.GetParent())
	assert.Contains(t, top.Files, fs.Item(emptied))
	assert.NotContains(t, top.Files, fs.Item(file))
	assert.Equal(t, count, top.GetItemCount())
	assert.Equal(t, size-100, top.GetSize())
	assert.Equal(t, usage-4096, top.GetUsage())
}

// A directory that cannot change, such as the top-dir analyzer's, is refused
// rather than changed in part.
func TestEntriesOfUnchangeableDir(t *testing.T) {
	top := &SimpleDir{SimpleFile: SimpleFile{Name: "top"}, Files: []SimpleFile{{Name: "file"}}}
	var file fs.Item
	for entry := range top.GetFiles(fs.SortByName, fs.SortAsc) {
		file = entry
	}

	assert.ErrorIs(t, RemoveEntry(top, file), ErrNotChangeable)
	assert.ErrorIs(t, ReplaceEntry(top, file, &File{Name: "file"}), ErrNotChangeable)
	assert.Len(t, top.Files, 1)
}

// A Node of another package among the entries computes its own totals.
func TestTotalsOfForeignNode(t *testing.T) {
	sub := nodeTestTree().sub
	sub.Size, sub.ItemCount = 0, 0
	foreign := &foreignNode{Node: sub}
	top := &Dir{File: &File{Name: "top"}, BasePath: "/", Files: fs.Files{foreign}}

	top.UpdateStats(false)

	assert.True(t, foreign.updated)
	assert.Equal(t, int64(3), top.GetItemCount())
	assert.Equal(t, int64(50), top.GetSize())
}

type foreignNode struct {
	Node
	updated bool
}

func (f *foreignNode) UpdateStats(filteringFiles bool) *fs.HardLinks {
	f.updated = true
	return f.Node.UpdateStats(filteringFiles)
}
