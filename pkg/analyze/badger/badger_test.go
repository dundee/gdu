package badger_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/analyze/badger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanAndLoad(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	path := filepath.Join(t.TempDir(), "test.badger")

	scanned := badger.CreateAnalyzer(path).AnalyzeDir(context.Background(), "test_dir", analyze.Options{})
	scanned.UpdateStats(false)

	loaded, err := badger.CreateAnalyzer(path).LoadAnalysis("test_dir")
	require.NoError(t, err)
	loaded.UpdateStats(false)
	assert.Equal(t, scanned.GetUsage(), loaded.GetUsage())
	assert.Equal(t, scanned.GetItemCount(), loaded.GetItemCount())
}
