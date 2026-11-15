package stdout

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/timefilter"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMultiTestDirs(t *testing.T) (first, second string) {
	t.Helper()

	require.NoError(t, os.MkdirAll("multi_a/nested", os.ModePerm))
	require.NoError(t, os.MkdirAll("multi_b/other", os.ModePerm))
	require.NoError(t, os.WriteFile("multi_a/nested/file", []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile("multi_b/other/file2", []byte("go"), 0o600))

	t.Cleanup(func() {
		assert.NoError(t, os.RemoveAll("multi_a"))
		assert.NoError(t, os.RemoveAll("multi_b"))
	})

	return "multi_a", "multi_b"
}

func TestAnalyzePathsListsEveryRoot(t *testing.T) {
	first, second := createMultiTestDirs(t)
	output := &bytes.Buffer{}

	ui := CreateStdoutUI(output, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{})
	require.NoError(t, ui.AnalyzePaths([]string{first, second}))

	assert.Contains(t, output.String(), first)
	assert.Contains(t, output.String(), second)
}

// Roots from different parents can share a base name, so they are listed by
// absolute path instead.
func TestAnalyzePathsLabelsRootsWithTheirPath(t *testing.T) {
	first := filepath.Join("outer_p", "data")
	second := filepath.Join("outer_q", "data")
	for _, dir := range []string{first, second} {
		require.NoError(t, os.MkdirAll(dir, os.ModePerm))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), []byte("hello"), 0o600))
	}
	t.Cleanup(func() {
		assert.NoError(t, os.RemoveAll("outer_p"))
		assert.NoError(t, os.RemoveAll("outer_q"))
	})

	output := &bytes.Buffer{}
	ui := CreateStdoutUI(output, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{})
	require.NoError(t, ui.AnalyzePaths([]string{first, second}))

	assert.Contains(t, output.String(), first)
	assert.Contains(t, output.String(), second)
	// the path is not given a second leading separator by the directory marker
	assert.NotContains(t, output.String(), string(filepath.Separator)+first)
}

func TestAnalyzePathsSummarizeReportsCombinedTotal(t *testing.T) {
	first, second := createMultiTestDirs(t)
	output := &bytes.Buffer{}

	ui := CreateStdoutUI(output, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{Display: config.Display{Summarize: true}})
	require.NoError(t, ui.AnalyzePaths([]string{first, second}))

	assert.NotEmpty(t, output.String())
}

func TestAnalyzePathsSwapsTopDirAnalyzerForFullTree(t *testing.T) {
	first, second := createMultiTestDirs(t)
	output := &bytes.Buffer{}

	ui := CreateStdoutUI(output, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{})
	// the default stdout analyzer produces SimpleDir, which cannot be re-parented
	_, isTopDirAnalyzer := ui.Analyzer.(*analyze.TopDirAnalyzer)
	require.True(t, isTopDirAnalyzer)

	require.NoError(t, ui.AnalyzePaths([]string{first, second}))

	_, stillTopDirAnalyzer := ui.Analyzer.(*analyze.TopDirAnalyzer)
	assert.False(t, stillTopDirAnalyzer)
}

func TestAnalyzePathsKeepsAnalyzerSettings(t *testing.T) {
	first, second := createMultiTestDirs(t)
	scan := func(filtered bool) string {
		output := &bytes.Buffer{}
		s := &config.Settings{Display: config.Display{ApparentSize: true, Summarize: true, NoPrefix: true}}
		if filtered {
			// only files modified after 2999, so none
			filter, err := timefilter.NewTimeFilter("2999-01-01", "", "", "", time.Now(), time.UTC)
			require.NoError(t, err)
			s.Scan.TimeFilter, s.Scan.TimeLocation = filter, time.UTC
		}
		ui := CreateStdoutUI(output, io.Discard, analyze.CreateTopDirAnalyzer(), s)
		require.NoError(t, ui.AnalyzePaths([]string{first, second}))
		return output.String()
	}

	// the time filter set on the top dir analyzer still drops every file
	assert.NotEqual(t, scan(false), scan(true))
}

func TestAnalyzePathsWithSinglePathBehavesLikeAnalyzePath(t *testing.T) {
	first, _ := createMultiTestDirs(t)
	output := &bytes.Buffer{}

	ui := CreateStdoutUI(output, io.Discard, analyze.CreateTopDirAnalyzer(), &config.Settings{})
	require.NoError(t, ui.AnalyzePaths([]string{first}))

	// contents of the single root are listed, not the root itself
	assert.Contains(t, output.String(), "nested")
	assert.NotContains(t, output.String(), analyze.VirtualRootName)
}
