package analyze

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
)

// ---------------------------------------------------------------------------
// Helpers to create test archives
// ---------------------------------------------------------------------------

// writeTarEntries writes a tiny but realistic directory structure into w.
func writeTarEntries(t *testing.T, w io.Writer) {
	t.Helper()
	tw := tar.NewWriter(w)

	entries := []struct {
		hdr     tar.Header
		content []byte
	}{
		{tar.Header{Typeflag: tar.TypeDir, Name: "subdir/", Mode: 0o755}, nil},
		{tar.Header{Typeflag: tar.TypeReg, Name: "test.txt", Size: 11, Mode: 0o644}, []byte("hello world")},
		{tar.Header{Typeflag: tar.TypeReg, Name: "subdir/nested.txt", Size: 6, Mode: 0o644}, []byte("nested")},
	}

	for _, e := range entries {
		require.NoError(t, tw.WriteHeader(&e.hdr))
		if e.content != nil {
			_, err := tw.Write(e.content)
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
}

func createTestTarFile(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	writeTarEntries(t, f)
}

func createTestTarGzFile(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	gw := gzip.NewWriter(f)
	writeTarEntries(t, gw)
	require.NoError(t, gw.Close())
}

func createTestTarBz2File(t *testing.T, path string) {
	t.Helper()
	// compress/bzip2 only provides a reader; use a pre-recorded bzip2-compressed
	// tar so we can test the reading path without an external encoder.
	// We build the plain tar in memory, then bzip2-compress it via a known
	// approach: write raw tar bytes, then BZ-compress them with a pure-Go writer.
	// Go stdlib has no bzip2 writer, so we create a minimal bzip2 stream by
	// piping through the `bzip2` command if available, otherwise skip.
	var buf bytes.Buffer
	writeTarEntries(t, &buf)

	// Try to use bzip2 command-line tool
	tmp := t.TempDir()
	rawTar := filepath.Join(tmp, "tmp.tar")
	require.NoError(t, os.WriteFile(rawTar, buf.Bytes(), 0o600))

	out, err := os.Create(path)
	require.NoError(t, err)
	defer out.Close()

	// Write a real bzip2 stream using a simple pure-Go BZ2 encoder shim.
	// Because the stdlib only provides a reader, we vendor a minimal BZ2 writer
	// here via dsnet/compress or simply write the file using pgzip. Instead, we
	// use a known-good approach: encode via Go's exec. If exec is unavailable we
	// create the file using a bzip2 block that wraps our raw bytes.
	//
	// Simplest portable approach: use compress/flate at the tar layer instead.
	// But since we need a real bzip2 file, we skip if bzip2 is unavailable.
	bzip2Cmd, lookErr := findBzip2Cmd()
	if lookErr != nil {
		t.Skip("bzip2 command not available, skipping .tar.bz2 test")
	}

	rawData, err := os.ReadFile(rawTar)
	require.NoError(t, err)
	compressed, err := bzip2Cmd(rawData)
	require.NoError(t, err)
	_, err = out.Write(compressed)
	require.NoError(t, err)
}

func createTestTarXzFile(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	xw, err := xz.NewWriter(f)
	require.NoError(t, err)
	writeTarEntries(t, xw)
	require.NoError(t, xw.Close())
}

// findBzip2Cmd returns a function that compresses data with bzip2, or an error
// if bzip2 is not available on the system.
func findBzip2Cmd() (func([]byte) ([]byte, error), error) {
	// We use os/exec but need to import it. To avoid adding an import only used
	// in tests, use a direct syscall approach via os/exec in a sub-function.
	return bzip2CompressFunc()
}

// ---------------------------------------------------------------------------
// isTarFile
// ---------------------------------------------------------------------------

func TestIsTarFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"archive.tar", true},
		{"archive.tar.gz", true},
		{"archive.tgz", true},
		{"archive.tar.bz2", true},
		{"archive.tbz2", true},
		{"archive.tar.xz", true},
		{"archive.txz", true},
		{"ARCHIVE.TAR", true},
		{"ARCHIVE.TAR.GZ", true},
		{"ARCHIVE.TGZ", true},
		{"ARCHIVE.TAR.BZ2", true},
		{"ARCHIVE.TBZZ2", false},
		{"ARCHIVE.TAR.XZ", true},
		{"ARCHIVE.TXZ", true},
		{"archive.zip", false},
		{"archive.jar", false},
		{"archive.gz", false},  // plain gzip, not a tarball
		{"archive.bz2", false}, // plain bzip2, not a tarball
		{"archive.xz", false},
		{"archive.txt", false},
		{"archive", false},
		{"", false},
	}

	for _, tc := range tests {
		result := isTarFile(tc.filename)
		assert.Equal(t, tc.expected, result, "filename: %s", tc.filename)
	}
}

// ---------------------------------------------------------------------------
// processTarFile – plain .tar
// ---------------------------------------------------------------------------

func TestProcessTarFile(t *testing.T) {
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar")
	createTestTarFile(t, tarPath)

	info, err := os.Stat(tarPath)
	require.NoError(t, err)

	td, err := processTarFile(tarPath, info)
	require.NoError(t, err)
	require.NotNil(t, td)

	assert.Equal(t, "test.tar", td.GetName())
	assert.Equal(t, rune('T'), td.GetFlag())
	assert.True(t, td.IsDir())
	assert.Equal(t, "TarDirectory", td.GetType())

	// Usage should equal the on-disk size
	assert.Equal(t, info.Size(), td.Usage)

	files := slices.Collect(td.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Greater(t, len(files), 0)

	foundText := false
	foundSubdir := false
	for _, f := range files {
		if f.GetName() == "test.txt" {
			foundText = true
			assert.False(t, f.IsDir())
			assert.Equal(t, "TarFile", f.GetType())
		}
		if f.GetName() == "subdir" {
			foundSubdir = true
			assert.True(t, f.IsDir())
			assert.Equal(t, "TarDirectory", f.GetType())
		}
	}
	assert.True(t, foundText, "should find test.txt")
	assert.True(t, foundSubdir, "should find subdir")
}

// ---------------------------------------------------------------------------
// processTarFile – .tar.gz
// ---------------------------------------------------------------------------

func TestProcessTarGzFile(t *testing.T) {
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar.gz")
	createTestTarGzFile(t, tarPath)

	info, err := os.Stat(tarPath)
	require.NoError(t, err)

	td, err := processTarFile(tarPath, info)
	require.NoError(t, err)
	require.NotNil(t, td)

	assert.Equal(t, "test.tar.gz", td.GetName())
	assert.True(t, td.IsDir())
	assert.Equal(t, "TarDirectory", td.GetType())
	assert.Equal(t, info.Size(), td.Usage)

	files := slices.Collect(td.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Greater(t, len(files), 0)
}

// ---------------------------------------------------------------------------
// processTarFile – .tgz alias
// ---------------------------------------------------------------------------

func TestProcessTgzFile(t *testing.T) {
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tgz")
	createTestTarGzFile(t, tarPath)

	info, err := os.Stat(tarPath)
	require.NoError(t, err)

	td, err := processTarFile(tarPath, info)
	require.NoError(t, err)
	require.NotNil(t, td)
	assert.Equal(t, "test.tgz", td.GetName())
}

// ---------------------------------------------------------------------------
// processTarFile – .tar.bz2
// ---------------------------------------------------------------------------

func TestProcessTarBz2File(t *testing.T) {
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar.bz2")
	createTestTarBz2File(t, tarPath) // skips if bzip2 unavailable

	info, err := os.Stat(tarPath)
	require.NoError(t, err)

	td, err := processTarFile(tarPath, info)
	require.NoError(t, err)
	require.NotNil(t, td)

	assert.Equal(t, "test.tar.bz2", td.GetName())
	assert.True(t, td.IsDir())
	assert.Equal(t, "TarDirectory", td.GetType())
}

// ---------------------------------------------------------------------------
// processTarFile – .tar.xz
// ---------------------------------------------------------------------------

func TestProcessTarXzFile(t *testing.T) {
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar.xz")
	createTestTarXzFile(t, tarPath)

	info, err := os.Stat(tarPath)
	require.NoError(t, err)

	td, err := processTarFile(tarPath, info)
	require.NoError(t, err)
	require.NotNil(t, td)

	assert.Equal(t, "test.tar.xz", td.GetName())
	assert.True(t, td.IsDir())
	assert.Equal(t, "TarDirectory", td.GetType())

	files := slices.Collect(td.GetFiles(fs.SortByName, fs.SortAsc))
	assert.Greater(t, len(files), 0)
}

// ---------------------------------------------------------------------------
// processTarFile – .txz alias
// ---------------------------------------------------------------------------

func TestProcessTxzFile(t *testing.T) {
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.txz")
	createTestTarXzFile(t, tarPath)

	info, err := os.Stat(tarPath)
	require.NoError(t, err)

	td, err := processTarFile(tarPath, info)
	require.NoError(t, err)
	require.NotNil(t, td)
	assert.Equal(t, "test.txz", td.GetName())
}

// ---------------------------------------------------------------------------
// ensureTarDirExists
// ---------------------------------------------------------------------------

func TestEnsureTarDirExists(t *testing.T) {
	tarPath := "/fake/archive.tar"

	rootDir := &TarDir{
		Dir: &Dir{
			File:  &File{Name: "archive.tar", Flag: 'T'},
			Files: make(fs.Files, 0),
		},
		tarPath: tarPath,
	}

	dirMap := make(map[string]*TarDir)
	dirMap[""] = rootDir

	ensureTarDirExists(dirMap, "a/b/c", tarPath, rootDir)

	assert.Contains(t, dirMap, "a")
	assert.Contains(t, dirMap, "a/b")
	assert.Contains(t, dirMap, "a/b/c")

	assert.Equal(t, rootDir, dirMap["a"].GetParent())
	assert.Equal(t, dirMap["a"], dirMap["a/b"].GetParent())
	assert.Equal(t, dirMap["a/b"], dirMap["a/b/c"].GetParent())
}

func TestEnsureTarDirExistsIdempotent(t *testing.T) {
	tarPath := "/fake/archive.tar"
	rootDir := &TarDir{
		Dir:     &Dir{File: &File{Name: "archive.tar", Flag: 'T'}, Files: make(fs.Files, 0)},
		tarPath: tarPath,
	}
	dirMap := map[string]*TarDir{"": rootDir}

	ensureTarDirExists(dirMap, "sub", tarPath, rootDir)
	first := dirMap["sub"]
	ensureTarDirExists(dirMap, "sub", tarPath, rootDir) // must be idempotent
	assert.Same(t, first, dirMap["sub"])
}

// ---------------------------------------------------------------------------
// TarFile / TarDir method coverage
// ---------------------------------------------------------------------------

func TestTarFileGetPath(t *testing.T) {
	tf := &TarFile{
		File:      &File{Name: "file.txt"},
		tarPath:   "/path/to/archive.tar",
		inTarPath: "dir/file.txt",
	}
	assert.Equal(t, "/path/to/archive.tar/dir/file.txt", tf.GetPath())
}

func TestTarFileGetType(t *testing.T) {
	tf := &TarFile{File: &File{Name: "x"}}
	assert.Equal(t, "TarFile", tf.GetType())
}

func TestTarFileEncodeJSON(t *testing.T) {
	tf := &TarFile{
		File:      &File{Name: "x.txt", Size: 42},
		tarPath:   "/a.tar",
		inTarPath: "x.txt",
	}
	var buf bytes.Buffer
	assert.NoError(t, tf.EncodeJSON(&buf, false))
	assert.NotEmpty(t, buf.String())
}

func TestTarDirGetType(t *testing.T) {
	td := &TarDir{Dir: &Dir{File: &File{Name: "sub"}}}
	assert.Equal(t, "TarDirectory", td.GetType())
}

func TestTarDirIsDir(t *testing.T) {
	td := &TarDir{Dir: &Dir{File: &File{Name: "sub"}}}
	assert.True(t, td.IsDir())
}

func TestTarDirEncodeJSON(t *testing.T) {
	td := &TarDir{
		Dir:     &Dir{File: &File{Name: "sub"}, Files: make(fs.Files, 0)},
		tarPath: "/a.tar",
	}
	var buf bytes.Buffer
	assert.NoError(t, td.EncodeJSON(&buf, true))
	assert.NotEmpty(t, buf.String())
}

func TestTarDirGetPathWithParent(t *testing.T) {
	parent := &TarDir{
		Dir:     &Dir{File: &File{Name: "parent"}},
		tarPath: "/a.tar",
	}
	child := &TarDir{
		Dir:     &Dir{File: &File{Name: "child"}},
		tarPath: "/a.tar",
	}
	child.Parent = parent

	assert.Equal(t, filepath.Join(parent.GetPath(), "child"), child.GetPath())
}

func TestTarDirGetPathWithoutParent(t *testing.T) {
	td := &TarDir{
		Dir:     &Dir{File: &File{Name: "root"}},
		tarPath: "/path/to/archive.tar",
	}
	assert.Equal(t, "/path/to/archive.tar", td.GetPath())
}

// ---------------------------------------------------------------------------
// Integration: SequentialAnalyzer with tar files
// ---------------------------------------------------------------------------

func TestSequentialAnalyzerWithTarFile(t *testing.T) {
	tmpDir := t.TempDir()
	createTestTarFile(t, filepath.Join(tmpDir, "test.tar"))

	a := CreateSeqAnalyzer()
	a.SetArchiveBrowsing(true)

	result := a.AnalyzeDir(tmpDir,
		func(string, string) bool { return false },
		func(string) bool { return false },
	)

	require.NotNil(t, result)

	var tarItem fs.Item
	for f := range result.GetFiles(fs.SortByName, fs.SortAsc) {
		if f.GetName() == "test.tar" {
			tarItem = f
			break
		}
	}
	require.NotNil(t, tarItem, "should find test.tar as a browsable directory")
	assert.True(t, tarItem.IsDir())
	assert.Equal(t, "TarDirectory", tarItem.GetType())

	count := 0
	for range tarItem.GetFiles(fs.SortByName, fs.SortAsc) {
		count++
	}
	assert.Greater(t, count, 0, "tar archive should contain browsable content")
}

func TestSequentialAnalyzerWithTarGzFile(t *testing.T) {
	tmpDir := t.TempDir()
	createTestTarGzFile(t, filepath.Join(tmpDir, "test.tar.gz"))

	a := CreateSeqAnalyzer()
	a.SetArchiveBrowsing(true)

	result := a.AnalyzeDir(tmpDir,
		func(string, string) bool { return false },
		func(string) bool { return false },
	)

	require.NotNil(t, result)

	var tarItem fs.Item
	for f := range result.GetFiles(fs.SortByName, fs.SortAsc) {
		if f.GetName() == "test.tar.gz" {
			tarItem = f
			break
		}
	}
	require.NotNil(t, tarItem, "should find test.tar.gz as a browsable directory")
	assert.True(t, tarItem.IsDir())
}

// ---------------------------------------------------------------------------
// Integration: ParallelAnalyzer with tar files
// ---------------------------------------------------------------------------

func TestParallelAnalyzerWithTarFile(t *testing.T) {
	tmpDir := t.TempDir()
	createTestTarFile(t, filepath.Join(tmpDir, "archive.tar"))

	a := CreateAnalyzer()
	a.SetArchiveBrowsing(true)

	result := a.AnalyzeDir(tmpDir,
		func(string, string) bool { return false },
		func(string) bool { return false },
	)

	require.NotNil(t, result)

	var tarItem fs.Item
	for f := range result.GetFiles(fs.SortByName, fs.SortAsc) {
		if f.GetName() == "archive.tar" {
			tarItem = f
			break
		}
	}
	require.NotNil(t, tarItem, "should find archive.tar")
	assert.True(t, tarItem.IsDir())
	assert.Equal(t, "TarDirectory", tarItem.GetType())
}

func TestParallelAnalyzerWithTarXzFile(t *testing.T) {
	tmpDir := t.TempDir()
	createTestTarXzFile(t, filepath.Join(tmpDir, "archive.tar.xz"))

	a := CreateAnalyzer()
	a.SetArchiveBrowsing(true)

	result := a.AnalyzeDir(tmpDir,
		func(string, string) bool { return false },
		func(string) bool { return false },
	)

	require.NotNil(t, result)

	var tarItem fs.Item
	for f := range result.GetFiles(fs.SortByName, fs.SortAsc) {
		if f.GetName() == "archive.tar.xz" {
			tarItem = f
			break
		}
	}
	require.NotNil(t, tarItem, "should find archive.tar.xz")
	assert.True(t, tarItem.IsDir())
}

// ---------------------------------------------------------------------------
// Error handling: non-existent / corrupt archive
// ---------------------------------------------------------------------------

func TestProcessTarFileNotFound(t *testing.T) {
	_, err := processTarFile("/no/such/file.tar", nil)
	assert.Error(t, err)
}

func TestProcessTarFileCorrupt(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "bad.tar")
	require.NoError(t, os.WriteFile(p, []byte("not a tar file at all"), 0o600))

	info, err := os.Stat(p)
	require.NoError(t, err)

	// A corrupt plain-tar should still open (it's a file), but Next() will fail.
	// processTarFile should propagate the error.
	_, err = processTarFile(p, info)
	assert.Error(t, err)
}

func TestProcessTarGzFileCorrupt(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "bad.tar.gz")
	require.NoError(t, os.WriteFile(p, []byte("not gzip"), 0o600))

	info, err := os.Stat(p)
	require.NoError(t, err)

	_, err = processTarFile(p, info)
	assert.Error(t, err)
}

func TestProcessTarXzFileCorrupt(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "bad.tar.xz")
	require.NoError(t, os.WriteFile(p, []byte("not xz"), 0o600))

	info, err := os.Stat(p)
	require.NoError(t, err)

	_, err = processTarFile(p, info)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Archive browsing disabled – tar file treated as regular file
// ---------------------------------------------------------------------------

func TestAnalyzerTarBrowsingDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	createTestTarFile(t, filepath.Join(tmpDir, "archive.tar"))

	a := CreateSeqAnalyzer()
	// archiveBrowsing is false by default

	result := a.AnalyzeDir(tmpDir,
		func(string, string) bool { return false },
		func(string) bool { return false },
	)

	require.NotNil(t, result)

	var tarItem fs.Item
	for f := range result.GetFiles(fs.SortByName, fs.SortAsc) {
		if f.GetName() == "archive.tar" {
			tarItem = f
			break
		}
	}
	require.NotNil(t, tarItem)
	assert.False(t, tarItem.IsDir(), "when browsing disabled, tar is a plain file")
}

// ---------------------------------------------------------------------------
// bzip2 compression helper (uses os/exec "bzip2")
// ---------------------------------------------------------------------------

func bzip2CompressFunc() (func([]byte) ([]byte, error), error) {
	return bzip2CompressWithCmd()
}

func bzip2CompressWithCmd() (func([]byte) ([]byte, error), error) {
	bzip2Path, err := exec.LookPath("bzip2")
	if err != nil {
		return nil, err
	}
	return func(data []byte) ([]byte, error) {
		cmd := exec.Command(bzip2Path, "--compress", "--stdout")
		cmd.Stdin = bytes.NewReader(data)
		return cmd.Output()
	}, nil
}

// Ensure compress/bzip2 package is used (for reading .tar.bz2)
var _ = bzip2.NewReader
