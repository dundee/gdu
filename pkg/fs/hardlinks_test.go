package fs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func linkAt(dir *analyze.Dir, name string) *analyze.File {
	return &analyze.File{Name: name, Parent: dir, Mli: 7}
}

func TestHardLinksCountsTheFirstLink(t *testing.T) {
	dir := &analyze.Dir{File: &analyze.File{Name: "/d"}}
	links := fs.NewHardLinks()
	key := fs.InodeKey{Dev: 1, Ino: 7}

	assert.True(t, links.Add(key, linkAt(dir, "a")))
	assert.False(t, links.Add(key, linkAt(dir, "b")))
	assert.True(t, links.Add(fs.InodeKey{Dev: 2, Ino: 7}, linkAt(dir, "c")))

	assert.Len(t, links.Links(key), 2)
	assert.Equal(t, "a", links.Links(key)[0].GetName())
	assert.Nil(t, (*fs.HardLinks)(nil).Links(key))
}

func TestHardLinksRemove(t *testing.T) {
	root := &analyze.Dir{File: &analyze.File{Name: "/r"}}
	sub := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: root}}
	key := fs.InodeKey{Dev: 1, Ino: 7}
	newLinks := func() *fs.HardLinks {
		links := fs.NewHardLinks()
		links.Add(key, linkAt(sub, "a"))
		links.Add(key, linkAt(root, "b"))
		links.Add(key, linkAt(root, "c"))
		return links
	}

	t.Run("a link not counted", func(t *testing.T) {
		links := newLinks()
		found, counted, next := links.Remove("/r/b", "/")
		assert.True(t, found)
		assert.False(t, counted)
		assert.Empty(t, next)
		assert.Len(t, links.Links(key), 2)
	})
	t.Run("the counted link", func(t *testing.T) {
		links := newLinks()
		found, counted, next := links.Remove("/r/sub/a", "/")
		assert.True(t, found)
		assert.True(t, counted)
		assert.Equal(t, "b", next[0].GetName())
	})
	t.Run("a directory holding the counted link", func(t *testing.T) {
		links := newLinks()
		found, counted, next := links.Remove("/r/sub", "/")
		assert.True(t, found)
		assert.True(t, counted)
		assert.Equal(t, "b", next[0].GetName())
	})
	t.Run("a path sharing a prefix", func(t *testing.T) {
		links := newLinks()
		found, _, _ := links.Remove("/r/su", "/")
		assert.False(t, found)
	})
	t.Run("every link", func(t *testing.T) {
		links := newLinks()
		found, counted, next := links.Remove("/r", "/")
		assert.True(t, found)
		assert.True(t, counted)
		assert.Empty(t, next)
		assert.Empty(t, links.Links(key))
	})
}
