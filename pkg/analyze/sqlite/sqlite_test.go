package sqlite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/analyze/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAnalyzerCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "test.db")

	_, err := sqlite.CreateAnalyzer(path)

	require.NoError(t, err)
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestScanAndLoad(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	path := filepath.Join(t.TempDir(), "test.db")

	analyzer, err := sqlite.CreateAnalyzer(path)
	require.NoError(t, err)
	scanned := analyzer.AnalyzeDir(context.Background(), "test_dir", analyze.Options{})
	scanned.UpdateStats(false)

	analyzer, err = sqlite.CreateAnalyzer(path)
	require.NoError(t, err)
	loaded, err := analyzer.LoadAnalysis("test_dir")
	require.NoError(t, err)
	assert.Equal(t, scanned.GetUsage(), loaded.GetUsage())
	assert.Equal(t, scanned.GetItemCount(), loaded.GetItemCount())
}
