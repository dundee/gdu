package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestScannedDirIncludesOwnBlocks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "d")
	assert.NoError(t, os.Mkdir(dir, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("x"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "b"), []byte("y"), 0o600))

	want := usageOf(t, dir) + usageOf(t, filepath.Join(dir, "a")) + usageOf(t, filepath.Join(dir, "b"))

	analyzer := CreateSeqAnalyzer()
	scanned := analyzer.AnalyzeDir(context.Background(), root, common.ScanOptions{}).(*Dir)
	scanned.UpdateStats(false)

	assert.Equal(t, want, scanned.Files[0].GetUsage())
}

// TestZeroBlockDirIsNotChargedABlock pins issue #393: a directory the
// filesystem allocates no block to (e.g. a small directory on XFS, or any on
// APFS or tmpfs) must not be charged one.
func TestZeroBlockDirIsNotChargedABlock(t *testing.T) {
	root := zeroBlockDir(t)
	assert.NoError(t, os.WriteFile(filepath.Join(root, "empty"), nil, 0o600))

	for name, analyzer := range map[string]common.Analyzer{
		"parallel": CreateAnalyzer(),
		"top-dir":  CreateTopDirAnalyzer(),
	} {
		scanned := analyzer.AnalyzeDir(context.Background(), root, common.ScanOptions{})
		scanned.UpdateStats(false)

		assert.Equal(t, int64(0), scanned.GetUsage(), name)
	}
}

// zeroBlockDir returns a new directory that occupies no disk blocks, or skips
// the test when no such filesystem is available.
func zeroBlockDir(t *testing.T) string {
	t.Helper()

	for _, parent := range []string{t.TempDir(), "/dev/shm"} {
		dir, err := os.MkdirTemp(parent, "gdu-zero-block")
		if err != nil {
			continue
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		if usageOf(t, dir) == 0 {
			return dir
		}
	}
	t.Skip("no filesystem with zero-block directories available")
	return ""
}
