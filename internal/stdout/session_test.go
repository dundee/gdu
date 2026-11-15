package stdout

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/session"
	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/analyze/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanObserverPrintsTheProgress(t *testing.T) {
	output := bytes.Buffer{}
	ui := CreateStdoutUI(&output, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})
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
	ui := CreateStdoutUI(&bytes.Buffer{}, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{})

	assert.Nil(t, ui.scanObserver())
}

// --read-from-storage prints the stored analysis, not the current state of
// the disk.
func TestReadFromStoragePrintsTheStoredAnalysis(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	db := filepath.Join(t.TempDir(), "gdu.sqlite")

	scanned := bytes.Buffer{}
	ui := CreateStdoutUI(&scanned, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{})
	analyzer, err := sqlite.CreateAnalyzer(db)
	require.NoError(t, err)
	ui.Analyzer = (analyzer)
	require.NoError(t, ui.AnalyzePath("test_dir", nil))

	require.NoError(t, os.WriteFile("test_dir/nested/added", make([]byte, 100000), 0o600))

	stored := bytes.Buffer{}
	ui = CreateStdoutUI(&stored, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{})
	analyzer, err = sqlite.CreateAnalyzer(db)
	require.NoError(t, err)
	ui.Analyzer = (analyzer)
	require.NoError(t, ui.ReadFromStorage("test_dir"))
	assert.Equal(t, scanned.String(), stored.String())

	assert.Error(t, ui.ReadFromStorage("test_dir/nested"))
}

func TestReadFromStorageNeedsDatabase(t *testing.T) {
	ui := CreateStdoutUI(&bytes.Buffer{}, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{})

	assert.ErrorIs(t, ui.ReadFromStorage("test_dir"), session.ErrNoDatabase)
}
