//go:build !windows && !plan9

package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// Every computation of the totals starts with a fresh hard-link index: one
// reused from an earlier one would find every hard link counted already and
// count none of them.
func TestHardLinksAreCountedOnceAcrossRescans(t *testing.T) {
	path := makeTree(t, "scan")
	require.NoError(t, os.Link(filepath.Join(path, "file"), filepath.Join(path, "dir", "link")))
	s := newSession(analyze.CreateAnalyzer())
	root := s.Scan(ctx, []string{path}, nil)
	usage := root.GetUsage()
	file := child(t, root, "file")
	require.NotZero(t, file.GetMultiLinkedInode())

	assert.Len(t, s.HardLinks(file.GetMultiLinkedInode()), 2)

	s.Rescan(ctx, child(t, root, "dir"), nil)

	assert.Equal(t, usage, root.GetUsage())
	assert.Len(t, s.HardLinks(file.GetMultiLinkedInode()), 2)

	s.Rescan(ctx, root, nil)
	assert.Equal(t, usage, s.Root().GetUsage())
	assert.Equal(t, fs.DisplayedItemCount(root), fs.DisplayedItemCount(s.Root()))
}
