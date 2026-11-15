package stdout

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/session"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
)

func TestScanObserverPrintsTheProgress(t *testing.T) {
	output := bytes.Buffer{}
	ui := CreateStdoutUI(&output, false, true, false, false, false, false, false, "", 0, false, 0)
	observe := ui.scanObserver()

	observe(session.Progress{Progress: analyze.Progress{ItemCount: 1234, TotalUsage: 2048}})
	assert.Contains(t, output.String(), "Scanning... Total items: 1,234 size: 2.0 KiB")

	observe(session.Progress{Phase: session.Finalizing})
	assert.Contains(t, output.String(), "Calculating disk usage...")

	output.Reset()
	ui.clearProgress()
	assert.NotContains(t, output.String(), "Scanning")
}

func TestScanObserverIsNilWithoutProgress(t *testing.T) {
	ui := CreateStdoutUI(&bytes.Buffer{}, false, false, false, false, false, false, false, "", 0, false, 0)

	assert.Nil(t, ui.scanObserver())
}

// --read-from-storage prints the stored analysis, not the current state of
// the disk.
func TestReadFromStoragePrintsTheStoredAnalysis(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	db := filepath.Join(t.TempDir(), "gdu.sqlite")

	scanned := bytes.Buffer{}
	ui := CreateStdoutUI(&scanned, false, false, false, false, false, false, false, "", 0, false, 0)
	analyzer, err := analyze.CreateSqliteAnalyzer(db)
	require.NoError(t, err)
	ui.SetAnalyzer(analyzer)
	require.NoError(t, ui.AnalyzePath("test_dir", nil))

	require.NoError(t, os.WriteFile("test_dir/nested/added", make([]byte, 100000), 0o600))

	stored := bytes.Buffer{}
	ui = CreateStdoutUI(&stored, false, false, false, false, false, false, false, "", 0, false, 0)
	analyzer, err = analyze.CreateSqliteAnalyzer(db)
	require.NoError(t, err)
	ui.SetAnalyzer(analyzer)
	require.NoError(t, ui.ReadFromStorage("test_dir"))
	assert.Equal(t, scanned.String(), stored.String())

	assert.Error(t, ui.ReadFromStorage("test_dir/nested"))
}

func TestReadFromStorageNeedsDatabase(t *testing.T) {
	ui := CreateStdoutUI(&bytes.Buffer{}, false, false, false, false, false, false, false, "", 0, false, 0)

	assert.ErrorIs(t, ui.ReadFromStorage("test_dir"), session.ErrNoDatabase)
}
