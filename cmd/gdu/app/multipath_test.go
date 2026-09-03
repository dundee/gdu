package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMultiTestDir creates two sibling directories that can be scanned
// together, and returns their names.
func createMultiTestDir(t *testing.T) (first, second string) {
	t.Helper()

	for _, dir := range []string{"multi_a/nested", "multi_b/other"} {
		require.NoError(t, os.MkdirAll(dir, os.ModePerm))
	}
	require.NoError(t, os.WriteFile("multi_a/nested/file", []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile("multi_b/other/file2", []byte("go"), 0o600))

	t.Cleanup(func() {
		assert.NoError(t, os.RemoveAll("multi_a"))
		assert.NoError(t, os.RemoveAll("multi_b"))
	})

	return "multi_a", "multi_b"
}

func TestResolvePathsDefaultsToCwd(t *testing.T) {
	a := &App{Flags: &Flags{}, Args: []string{}}

	paths, err := a.resolvePaths()

	cwd, cwdErr := filepath.Abs(".")
	require.NoError(t, cwdErr)
	assert.NoError(t, err)
	assert.Equal(t, []string{cwd}, paths)
}

func TestResolvePathsMakesArgsAbsolute(t *testing.T) {
	a := &App{Flags: &Flags{}, Args: []string{"multi_a", "multi_b"}}

	paths, err := a.resolvePaths()

	assert.NoError(t, err)
	require.Len(t, paths, 2)
	for _, path := range paths {
		assert.True(t, filepath.IsAbs(path), "expected absolute path, got %s", path)
	}
}

func TestResolvePathsDropsDuplicates(t *testing.T) {
	a := &App{Flags: &Flags{}, Args: []string{"multi_a", "./multi_a", "multi_b"}}

	paths, err := a.resolvePaths()

	assert.NoError(t, err)
	assert.Len(t, paths, 2)
}

func TestResolvePathsRejectsNestedPaths(t *testing.T) {
	a := &App{Flags: &Flags{}, Args: []string{"multi_a", "multi_a/nested"}}

	_, err := a.resolvePaths()

	assert.ErrorContains(t, err, "would count it twice")
}

func TestResolvePathsRejectsNestedPathsRegardlessOfOrder(t *testing.T) {
	a := &App{Flags: &Flags{}, Args: []string{"multi_a/nested", "multi_a"}}

	_, err := a.resolvePaths()

	assert.ErrorContains(t, err, "would count it twice")
}

func TestResolvePathsAllowsSiblingsWithSharedPrefix(t *testing.T) {
	a := &App{Flags: &Flags{}, Args: []string{"multi_a", "multi_ab"}}

	paths, err := a.resolvePaths()

	assert.NoError(t, err)
	assert.Len(t, paths, 2)
}

func TestIsSubPath(t *testing.T) {
	root := string(filepath.Separator)

	assert.True(t, isSubPath(filepath.Join(root, "a", "b"), filepath.Join(root, "a")))
	assert.True(t, isSubPath(filepath.Join(root, "a", "..foo"), filepath.Join(root, "a")))
	assert.False(t, isSubPath(filepath.Join(root, "a"), filepath.Join(root, "a")))
	assert.False(t, isSubPath(filepath.Join(root, "ab"), filepath.Join(root, "a")))
	assert.False(t, isSubPath(filepath.Join(root, "a"), filepath.Join(root, "a", "b")))
}

func TestAnalyzeMultiplePaths(t *testing.T) {
	first, second := createMultiTestDir(t)

	out, err := runApp(
		&Flags{LogFile: "/dev/null"},
		[]string{first, second},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.NoError(t, err)
	// both scanned roots are listed under the virtual top level dir
	assert.Contains(t, out, first)
	assert.Contains(t, out, second)
}

func TestAnalyzeMultiplePathsSummarize(t *testing.T) {
	first, second := createMultiTestDir(t)

	out, err := runApp(
		&Flags{LogFile: "/dev/null", Summarize: true, NonInteractive: true},
		[]string{first, second},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestAnalyzeSinglePathIsUnchangedByMultiPathSupport(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null"},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.NoError(t, err)
	assert.Contains(t, out, "nested")
	// no virtual root is introduced for a single directory
	assert.NotContains(t, out, "(multiple)")
}

func TestMultiplePathsRejectedForExport(t *testing.T) {
	first, second := createMultiTestDir(t)

	_, err := runApp(
		&Flags{LogFile: "/dev/null", OutputFile: "/dev/null"},
		[]string{first, second},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.ErrorContains(t, err, "--output-file accepts only one directory")
}

func TestMultiplePathsRejectedForStorage(t *testing.T) {
	first, second := createMultiTestDir(t)

	_, err := runApp(
		&Flags{LogFile: "/dev/null", DbPath: filepath.Join(t.TempDir(), "gdu.db")},
		[]string{first, second},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.ErrorContains(t, err, "--db accepts only one directory")
}
