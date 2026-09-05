//go:build linux

package report

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/stretchr/testify/assert"
)

func TestReadFromStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	const storagePath = "/tmp/badger-test2"
	defer func() {
		err := os.RemoveAll(storagePath)
		if err != nil {
			panic(err)
		}
	}()

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, true, false, 0, 0, false, nil)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.SetAnalyzer(analyze.CreateStoredAnalyzer(storagePath))
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.ReadFromStorage(storagePath, "test_dir")

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
}

func TestAnalyzePathWithSummarizeFromStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var plain, plainReport bytes.Buffer
	plainUI := CreateExportUI(&plain, &plainReport, false, false, false, 0, 0, true, nil)
	plainUI.SetIgnoreDirPaths([]string{"/xxx"})
	assert.Nil(t, plainUI.AnalyzePath("test_dir", nil))

	var output, reportOutput bytes.Buffer
	ui := CreateExportUI(&output, &reportOutput, false, false, false, 0, 0, true, nil)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.SetAnalyzer(analyze.CreateStoredAnalyzer(t.TempDir()))
	assert.Nil(t, ui.AnalyzePath("test_dir", nil))

	// stored dirs are not *analyze.Dir, so the summary used to report zeroes
	assert.NotContains(t, reportOutput.String(), `"asize":0,"dsize":0,"items":0`)
	assert.Equal(t, summaryBody(plainReport.String()), summaryBody(reportOutput.String()))
}

// summaryBody drops the export header, whose timestamp differs between runs.
func summaryBody(report string) string {
	_, body, _ := strings.Cut(report, "\n")
	return body
}

func TestAnalyzePathWithDepthFromStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	var plain, plainReport bytes.Buffer
	plainUI := CreateExportUI(&plain, &plainReport, false, false, false, 0, 1, false, nil)
	plainUI.SetIgnoreDirPaths([]string{"/xxx"})
	assert.Nil(t, plainUI.AnalyzePath("test_dir", nil))

	var output, reportOutput bytes.Buffer
	ui := CreateExportUI(&output, &reportOutput, false, false, false, 0, 1, false, nil)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.SetAnalyzer(analyze.CreateStoredAnalyzer(t.TempDir()))
	assert.Nil(t, ui.AnalyzePath("test_dir", nil))

	// stored dirs load their children lazily, so the depth limit used to export
	// them without stats and without descending into them
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
	assert.NotContains(t, reportOutput.String(), `"asize":0,"dsize":0,"items":0`)
	assert.Equal(t, summaryBody(plainReport.String()), summaryBody(reportOutput.String()))
}

func TestReadFromStorageWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	const storagePath = "/tmp/badger-test3"

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, false, false, 0, 0, false, nil)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.ReadFromStorage(storagePath, "test_dir")

	assert.ErrorContains(t, err, "Key not found")
}
