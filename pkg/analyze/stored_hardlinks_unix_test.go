//go:build !windows && !plan9

package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/common"
)

// Removing the links of a file from a stored tree keeps the file counted
// while a link is left, in the tree and in the database.
func TestStoredHardLinkRemoval(t *testing.T) {
	path := t.TempDir()
	writeSized(t, filepath.Join(path, "a", "f"), 40000, zeroTime)
	require.NoError(t, os.MkdirAll(filepath.Join(path, "b"), 0o755))
	require.NoError(t, os.Link(filepath.Join(path, "a", "f"), filepath.Join(path, "b", "g")))
	fileUsage := usageOf(t, filepath.Join(path, "a", "f"))
	db := filepath.Join(t.TempDir(), "gdu.badger")
	root := CreateStoredAnalyzer(db).AnalyzeDir(context.Background(), path, common.ScanOptions{})
	root.UpdateStats(false)
	usage, count := root.GetUsage(), root.GetItemCount()

	a := entry(t, root, "a")
	a.RemoveFile(entry(t, a, "f"))

	assert.Equal(t, usage, root.GetUsage(), "g is still there")
	assert.Equal(t, count-1, root.GetItemCount())
	assert.GreaterOrEqual(t, entry(t, root, "b").GetUsage(), fileUsage, "g is counted now")

	loaded, err := CreateStoredAnalyzer(db).LoadAnalysis(path)
	require.NoError(t, err)
	assert.Equal(t, usage, loaded.GetUsage())
	assert.Equal(t, count-1, loaded.GetItemCount())

	b := entry(t, root, "b")
	b.RemoveFile(entry(t, b, "g"))

	assert.Equal(t, usage-fileUsage, root.GetUsage())
}
