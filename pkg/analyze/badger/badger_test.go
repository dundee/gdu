package badger_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/analyze/badger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanAndLoad(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	path := filepath.Join(t.TempDir(), "test.badger")

	scanner := badger.CreateAnalyzer(path)
	scanned := scanner.AnalyzeDir(context.Background(), "test_dir", analyze.Options{})
	scanned.UpdateStats(false)
	require.NoError(t, scanner.Close())

	loader := badger.CreateAnalyzer(path)
	defer loader.Close()
	loaded, err := loader.LoadAnalysis("test_dir")
	require.NoError(t, err)
	loaded.UpdateStats(false)
	assert.Equal(t, scanned.GetUsage(), loaded.GetUsage())
	assert.Equal(t, scanned.GetItemCount(), loaded.GetItemCount())
}
