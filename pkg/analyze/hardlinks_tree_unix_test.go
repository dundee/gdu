//go:build !windows && !plan9

package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// scanLinkedTree scans root/{a/f, b/g, c/plain}, f and g being two links to
// one file, and computes its totals.
func scanLinkedTree(t *testing.T) (root *Dir, fileUsage int64) {
	t.Helper()
	path := t.TempDir()
	writeSized(t, filepath.Join(path, "a", "f"), 40000, zeroTime)
	writeSized(t, filepath.Join(path, "c", "plain"), 1000, zeroTime)
	require.NoError(t, os.MkdirAll(filepath.Join(path, "b"), 0o755))
	require.NoError(t, os.Link(filepath.Join(path, "a", "f"), filepath.Join(path, "b", "g")))

	root = CreateAnalyzer().AnalyzeDir(context.Background(), path, common.ScanOptions{}).(*Dir)
	updateTotals(root)
	return root, usageOf(t, filepath.Join(path, "a", "f"))
}

func childDir(t *testing.T, dir *Dir, name string) *Dir {
	t.Helper()
	for _, item := range dir.Files {
		if item.GetName() == name {
			return item.(*Dir)
		}
	}
	t.Fatalf("no %s in %s", name, dir.GetPath())
	return nil
}

// countedLink returns the directory whose link to the shared file is counted
// in the totals, and the other one.
func countedLink(t *testing.T, root *Dir, fileUsage int64) (counted, other *Dir) {
	t.Helper()
	a, b := childDir(t, root, "a"), childDir(t, root, "b")
	if a.GetUsage() >= fileUsage {
		return a, b
	}
	require.GreaterOrEqual(t, b.GetUsage(), fileUsage, "one link is counted")
	return b, a
}

// Removing a link that was not counted removes an item, not disk usage: the
// file is still there through the other link.
func TestRemovingUncountedHardLinkKeepsUsage(t *testing.T) {
	root, fileUsage := scanLinkedTree(t)
	_, other := countedLink(t, root, fileUsage)
	usage, otherUsage, count := root.GetUsage(), other.GetUsage(), root.GetItemCount()

	other.RemoveFile(other.Files[0])

	assert.Equal(t, usage, root.GetUsage())
	assert.Equal(t, otherUsage, other.GetUsage())
	assert.Equal(t, count-1, root.GetItemCount())
}

// Removing the link that was counted moves the count to the remaining link.
func TestRemovingCountedHardLinkMovesItsUsage(t *testing.T) {
	root, fileUsage := scanLinkedTree(t)
	counted, other := countedLink(t, root, fileUsage)
	usage, count := root.GetUsage(), root.GetItemCount()
	countedUsage, otherUsage := counted.GetUsage(), other.GetUsage()

	counted.RemoveFile(counted.Files[0])

	assert.Equal(t, usage, root.GetUsage())
	assert.Equal(t, count-1, root.GetItemCount())
	assert.Equal(t, countedUsage-fileUsage, counted.GetUsage())
	assert.Equal(t, otherUsage+fileUsage, other.GetUsage())
}

// Removing every link removes the file's usage once.
func TestRemovingEveryHardLinkRemovesUsageOnce(t *testing.T) {
	root, fileUsage := scanLinkedTree(t)
	usage := root.GetUsage()
	a, b := childDir(t, root, "a"), childDir(t, root, "b")

	a.RemoveFile(a.Files[0])
	b.RemoveFile(b.Files[0])

	assert.Equal(t, usage-fileUsage, root.GetUsage())
	assert.Equal(t, a.OwnUsage, a.GetUsage())
	assert.Equal(t, b.OwnUsage, b.GetUsage())
}

var zeroTime time.Time

// updateTotals computes the totals of the tree topped by root.
func updateTotals(root fs.Item) {
	root.UpdateStats(false)
}
