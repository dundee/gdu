package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMultiTestDirs creates two sibling directories to be scanned together.
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

// createNestedTestDirs creates two directories that share a base name but live
// under different parents.
func createNestedTestDirs(t *testing.T) (first, second string) {
	first = filepath.Join("outer_p", "data")
	second = filepath.Join("outer_q", "data")

	for _, dir := range []string{first, second} {
		require.NoError(t, os.MkdirAll(dir, os.ModePerm))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), []byte("hello"), 0o600))
	}

	t.Cleanup(func() {
		assert.NoError(t, os.RemoveAll("outer_p"))
		assert.NoError(t, os.RemoveAll("outer_q"))
	})

	return first, second
}

// analyzeMultiplePaths runs a multi-root scan to completion and flushes the
// queued UI updates, as the TUI tests do for single-path scans.
func analyzeMultiplePaths(t *testing.T, paths []string) *UI {
	t.Helper()

	simScreen := testapp.CreateSimScreen()
	t.Cleanup(simScreen.Fini)

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.done = make(chan struct{})

	require.NoError(t, ui.AnalyzePaths(paths))

	<-ui.done
	for _, draw := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		draw()
	}

	return ui
}

func TestAnalyzePathsGroupsRootsUnderVirtualDir(t *testing.T) {
	first, second := createMultiTestDirs(t)

	ui := analyzeMultiplePaths(t, []string{first, second})

	assert.True(t, analyze.IsVirtualRootDir(ui.currentDir))
	assert.Equal(t, analyze.VirtualRootName, ui.topDirPath)
	// no "/.." row is shown at the top, so the rows are exactly the two roots
	assert.Equal(t, 2, ui.table.GetRowCount())

	names := []string{ui.table.GetCell(0, 0).Text, ui.table.GetCell(1, 0).Text}
	assert.Contains(t, names[0]+names[1], first)
	assert.Contains(t, names[0]+names[1], second)
}

// Roots from different parents can share a base name, so rows under the virtual
// top level dir are labelled with the absolute path instead.
func TestRootsAreLabelledWithTheirAbsolutePath(t *testing.T) {
	first, second := createNestedTestDirs(t)

	ui := analyzeMultiplePaths(t, []string{first, second})

	rows := ui.table.GetCell(0, 0).Text + "\n" + ui.table.GetCell(1, 0).Text
	assert.Contains(t, rows, first)
	assert.Contains(t, rows, second)
	// the path is not given a second leading separator by the directory marker
	assert.NotContains(t, rows, string(filepath.Separator)+first)
}

func TestRootsKeepBaseNamesBelowTheVirtualDir(t *testing.T) {
	first, second := createMultiTestDirs(t)
	ui := analyzeMultiplePaths(t, []string{first, second})

	// descend into the first root; its children are ordinary rows again
	ui.table.Select(0, 0)
	ui.handleRight()

	assert.False(t, analyze.IsVirtualRootDir(ui.currentDir))
	assert.Contains(t, ui.table.GetCell(1, 0).Text, "/nested")
}

func TestFilteringAtVirtualRootMatchesThePathShown(t *testing.T) {
	first, second := createNestedTestDirs(t)
	ui := analyzeMultiplePaths(t, []string{first, second})

	// "outer_p" appears only in the path, never in the base name
	ui.filterValue = "outer_p"
	ui.showDir()

	assert.Equal(t, 1, ui.table.GetRowCount())
	assert.Contains(t, ui.table.GetCell(0, 0).Text, first)
}

func TestAnalyzePathsKeepsRealPathsOfRoots(t *testing.T) {
	first, second := createMultiTestDirs(t)

	ui := analyzeMultiplePaths(t, []string{first, second})

	paths := ui.scannedRootPaths()
	assert.ElementsMatch(t, []string{first, second}, paths)
}

func TestAnalyzePathsSumsRootStats(t *testing.T) {
	first, second := createMultiTestDirs(t)

	ui := analyzeMultiplePaths(t, []string{first, second})

	var rootSize int64
	for item := range ui.currentDir.GetFiles(0, 0) {
		rootSize += item.GetSize()
	}
	assert.Equal(t, rootSize, ui.currentDir.GetSize())
}

func TestAnalyzePathsWithSinglePathHasNoVirtualDir(t *testing.T) {
	first, _ := createMultiTestDirs(t)

	ui := analyzeMultiplePaths(t, []string{first})

	assert.False(t, analyze.IsVirtualRootDir(ui.currentDir))
	assert.Equal(t, first, ui.currentDir.GetName())
}

func TestEnteringAndLeavingARootUnderTheVirtualDir(t *testing.T) {
	first, second := createMultiTestDirs(t)
	ui := analyzeMultiplePaths(t, []string{first, second})

	// enter the first root
	ui.table.Select(0, 0)
	enteredName := ui.currentDir.GetName()
	ui.handleRight()
	assert.NotEqual(t, enteredName, ui.currentDir.GetName())
	assert.False(t, analyze.IsVirtualRootDir(ui.currentDir))

	// and back out to the virtual root via the "/.." row
	ui.handleLeft()
	assert.True(t, analyze.IsVirtualRootDir(ui.currentDir))
}

func TestHandleLeftAtVirtualRootDoesNotBrowseParent(t *testing.T) {
	first, second := createMultiTestDirs(t)
	ui := analyzeMultiplePaths(t, []string{first, second})
	ui.SetBrowseParentDirs()

	ui.handleLeft()

	// the virtual root has no parent on disk, so nothing may be rescanned
	assert.True(t, analyze.IsVirtualRootDir(ui.currentDir))
	assert.Equal(t, analyze.VirtualRootName, ui.topDirPath)
}

func TestRescanAtVirtualRootRescansAllRoots(t *testing.T) {
	first, second := createMultiTestDirs(t)
	ui := analyzeMultiplePaths(t, []string{first, second})

	ui.done = make(chan struct{})
	ui.rescanDir()
	<-ui.done
	for _, draw := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		draw()
	}

	assert.True(t, analyze.IsVirtualRootDir(ui.currentDir))
	assert.ElementsMatch(t, []string{first, second}, ui.scannedRootPaths())
}

func TestExportAtVirtualRootIsRejected(t *testing.T) {
	first, second := createMultiTestDirs(t)
	ui := analyzeMultiplePaths(t, []string{first, second})

	ui.exportAnalysis()

	assert.Contains(t, ui.header.GetText(false), "Export is not supported")
	assert.False(t, ui.pages.HasPage("exporting"))
}

func TestSpawnShellAtVirtualRootIsRejected(t *testing.T) {
	first, second := createMultiTestDirs(t)
	ui := analyzeMultiplePaths(t, []string{first, second})
	called := false
	ui.exec = func(argv0 string, argv, envv []string) error {
		called = true
		return nil
	}

	assert.Nil(t, ui.keyPressed(tcell.NewEventKey(tcell.KeyRune, 'b', 0)))

	assert.False(t, called)
	assert.Contains(t, ui.header.GetText(false), "Shell cannot be spawned")
}
