package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Re-exporting a report into itself fails, as reading a report while
// exporting is not possible; the report must survive that.
func TestOutputFileKeptWhenTheRunFails(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "keep.json")
	content := []byte(`[1,2,{"progname":"gdu","progver":"development","timestamp":1626806293},[{"name":"/dir"}]]`)
	require.NoError(t, os.WriteFile(report, content, 0o600))

	_, err := runApp(&Flags{LogFile: "/dev/null", InputFile: report, OutputFile: report},
		[]string{}, false, testdev.DevicesInfoGetterMock{})
	assert.ErrorContains(t, err, "reading analysis is not possible while exporting")

	_, err = runApp(&Flags{LogFile: "/dev/null", OutputFile: filepath.Join(dir, "new.json"), ShowDisks: true},
		[]string{}, false, testdev.DevicesInfoGetterMock{})
	assert.Error(t, err)

	after, err := os.ReadFile(report)
	require.NoError(t, err)
	assert.Equal(t, content, after)
	assertOnlyFiles(t, dir, "keep.json")
}

func TestOutputFileReplaced(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(report, []byte("old"), 0o640))

	_, err := runApp(&Flags{LogFile: "/dev/null", OutputFile: report},
		[]string{"test_dir"}, false, testdev.DevicesInfoGetterMock{})

	require.NoError(t, err)
	content, err := os.ReadFile(report)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"name":"nested"`)
	assertOnlyFiles(t, dir, "report.json")
	if runtime.GOOS != "windows" {
		info, err := os.Stat(report)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o640), info.Mode().Perm(), "the permissions of the replaced file are kept")
	}
}

func TestOutputFileInMissingDirFailsBeforeScanning(t *testing.T) {
	_, err := runApp(&Flags{LogFile: "/dev/null", OutputFile: filepath.Join(t.TempDir(), "missing", "report.json")},
		[]string{"/nonexistent-dir-to-scan"}, false, testdev.DevicesInfoGetterMock{})

	assert.ErrorContains(t, err, "opening output file")
}

func TestOutputFileFailedWriteKeepsTheFile(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(report, []byte("old"), 0o600))
	out, err := newOutputFile(report)
	require.NoError(t, err)

	_, err = out.Write([]byte("half"))
	require.NoError(t, err)
	out.err = errors.New("disk full")

	assert.ErrorContains(t, out.Close(), "disk full")
	content, err := os.ReadFile(report)
	require.NoError(t, err)
	assert.Equal(t, "old", string(content))
	assertOnlyFiles(t, dir, "report.json")
}

func TestOutputFileWrittenThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	link := filepath.Join(dir, "link.json")
	require.NoError(t, os.Symlink(target, link))

	out, err := newOutputFile(link)
	require.NoError(t, err)
	_, err = out.Write([]byte("new"))
	require.NoError(t, err)
	require.NoError(t, out.Close())

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(content))
	linkInfo, err := os.Lstat(link)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, linkInfo.Mode().Type(), "the link stays a link")
}

func assertOnlyFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	assert.ElementsMatch(t, names, got)
}

func skipUnlessPermissionsApply(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs directory permissions that apply to the user")
	}
}

func TestOutputFileInUnwritableDirFailsBeforeScanning(t *testing.T) {
	skipUnlessPermissionsApply(t)
	dir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.Mkdir(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := runApp(&Flags{LogFile: "/dev/null", OutputFile: filepath.Join(dir, "report.json")},
		[]string{"/nonexistent-dir-to-scan"}, false, testdev.DevicesInfoGetterMock{})

	assert.ErrorContains(t, err, "opening output file")
	assert.ErrorContains(t, err, "permission denied")
}

// A file the user may write, in a directory they may not, cannot be replaced
// by renaming; it is written in place, as it was before.
func TestOutputFileInUnwritableDirWrittenInPlace(t *testing.T) {
	skipUnlessPermissionsApply(t)
	fin := testdir.CreateTestDir()
	defer fin()
	dir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.Mkdir(dir, 0o700))
	report := filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(report, []byte("old"), 0o600))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := runApp(&Flags{LogFile: "/dev/null", OutputFile: report},
		[]string{"test_dir"}, false, testdev.DevicesInfoGetterMock{})

	require.NoError(t, err)
	content, err := os.ReadFile(report)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"name":"nested"`)
	assertOnlyFiles(t, dir, "report.json")
}

// Checking that the file can be written changes nothing.
func TestCheckWritableChangesNothing(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(report, []byte("old"), 0o600))

	require.NoError(t, checkWritable(report))
	require.NoError(t, checkWritable(filepath.Join(dir, "new.json")))

	content, err := os.ReadFile(report)
	require.NoError(t, err)
	assert.Equal(t, "old", string(content))
	assertOnlyFiles(t, dir, "report.json")
}
