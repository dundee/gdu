//go:build !windows && !plan9

package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/pkg/analyze"
)

// The session keeps what its last scan could not read, sorted by path.
func TestReadErrorsOfTheLastScan(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	root := makeTree(t, "root")
	var locked []string
	for _, name := range []string{"z", "a"} {
		dir := filepath.Join(root, name)
		require.NoError(t, os.Mkdir(dir, 0o755))
		require.NoError(t, os.Chmod(dir, 0o000))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		locked = append(locked, dir)
	}
	s := newSession(analyze.CreateAnalyzer())

	s.Scan(context.Background(), []string{root}, nil)

	errs := s.ReadErrors()
	require.Len(t, errs, 2)
	assert.Equal(t, locked[1], errs[0].Path)
	assert.Equal(t, locked[0], errs[1].Path)

	s.Scan(context.Background(), []string{filepath.Join(root, "dir")}, nil)
	assert.Empty(t, s.ReadErrors(), "a scan reports only its own")
}
