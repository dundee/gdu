//go:build !windows && !plan9

package analyze

import (
	"context"
	iofs "io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeLockedTree returns a tree holding a directory nobody can list, and the
// directory.
func makeLockedTree(t *testing.T) (root, locked string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	root = t.TempDir()
	locked = filepath.Join(root, "locked")
	require.NoError(t, os.MkdirAll(filepath.Join(locked, "inner"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	return root, locked
}

// Every analyzer tells what it could not read, so a frontend can say so.
func TestScanTellsWhatItCouldNotRead(t *testing.T) {
	root, locked := makeLockedTree(t)

	analyzers := append([]parityAnalyzer{
		{name: "parallel", analyzer: func(*testing.T) Analyzer { return CreateAnalyzer() }},
		{name: "top-dir", analyzer: func(*testing.T) Analyzer { return CreateTopDirAnalyzer() }},
	}, parityAnalyzers()...)
	for _, tc := range analyzers {
		t.Run(tc.name, func(t *testing.T) {
			var (
				mu   sync.Mutex
				errs []ReadError
			)
			tree := tc.analyzer(t).AnalyzeDir(context.Background(), root, Options{
				OnReadError: func(e ReadError) {
					mu.Lock()
					defer mu.Unlock()
					errs = append(errs, e)
				},
			})
			tree.UpdateStats(false)

			require.Len(t, errs, 1)
			assert.Equal(t, locked, errs[0].Path)
			assert.True(t, errs[0].Dir)
			assert.ErrorIs(t, errs[0].Err, iofs.ErrPermission)
		})
	}
}
