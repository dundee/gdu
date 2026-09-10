package report

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/stretchr/testify/assert"
)

// assertStorageExportMatchesPlain exports test_dir twice — once with the
// default in-memory analyzer and once with the given storage-backed analyzer —
// and requires both report bodies to be equal. Stored dirs are not
// *analyze.Dir, so --summarize and --depth used to export them with zeroed
// stats, a bare root name, and (for --depth) without descending into the
// lazily loaded children. It returns the storage-backed report for extra
// assertions.
func assertStorageExportMatchesPlain(
	t *testing.T, analyzer common.Analyzer, depth int, summarize bool,
) string {
	t.Helper()

	fin := testdir.CreateTestDir()
	defer fin()

	// The absolute path pins the exported root name: it is built from
	// BasePath, which is not on the fs.Item interface and used to be dropped
	// for stored roots.
	path, err := filepath.Abs("test_dir")
	assert.Nil(t, err)

	var plain, plainReport bytes.Buffer
	plainUI := CreateExportUI(&plain, &plainReport, false, false, false, 0, depth, summarize, nil)
	plainUI.SetIgnoreDirPaths([]string{"/xxx"})
	assert.Nil(t, plainUI.AnalyzePath(path, nil))

	var output, reportOutput bytes.Buffer
	ui := CreateExportUI(&output, &reportOutput, false, false, false, 0, depth, summarize, nil)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.SetAnalyzer(analyzer)
	assert.Nil(t, ui.AnalyzePath(path, nil))

	rootName, err := json.Marshal(path)
	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":`+string(rootName))
	assert.NotContains(t, reportOutput.String(), `"asize":0,"dsize":0,"items":0`)
	assert.Equal(t, reportWithoutHeader(plainReport.String()), reportWithoutHeader(reportOutput.String()))

	return reportOutput.String()
}

// reportWithoutHeader drops the export header, whose timestamp differs between runs.
func reportWithoutHeader(report string) string {
	_, body, _ := strings.Cut(report, "\n")
	return body
}

func createSqliteAnalyzer(t *testing.T) common.Analyzer {
	t.Helper()
	analyzer, err := analyze.CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Skipf("sqlite storage not available: %v", err)
	}
	return analyzer
}

func TestAnalyzePathWithSummarizeFromStorage(t *testing.T) {
	assertStorageExportMatchesPlain(t, analyze.CreateStoredAnalyzer(t.TempDir()), 0, true)
}

func TestAnalyzePathWithDepthFromStorage(t *testing.T) {
	report := assertStorageExportMatchesPlain(t, analyze.CreateStoredAnalyzer(t.TempDir()), 1, false)
	assert.Contains(t, report, `"name":"nested"`)
}

func TestAnalyzePathWithSummarizeFromSqliteStorage(t *testing.T) {
	assertStorageExportMatchesPlain(t, createSqliteAnalyzer(t), 0, true)
}

func TestAnalyzePathWithDepthFromSqliteStorage(t *testing.T) {
	report := assertStorageExportMatchesPlain(t, createSqliteAnalyzer(t), 1, false)
	assert.Contains(t, report, `"name":"nested"`)
}
