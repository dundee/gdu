package analyze

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

func tarOf(t *testing.T, entries int) []byte {
	t.Helper()
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	for i := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: fmt.Sprintf("f%d", i), Mode: 0o644}))
	}
	require.NoError(t, tw.Close())
	return out.Bytes()
}

// xzAskingFor compresses data into a valid xz file whose block header asks
// the decoder for a dictionary of dict bytes.
func xzAskingFor(t *testing.T, data []byte, dict int64) []byte {
	t.Helper()
	var out bytes.Buffer
	w, err := xz.WriterConfig{DictCap: 1 << 20}.NewWriter(&out)
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	file := out.Bytes()
	const streamHeader = 12
	header := file[streamHeader : streamHeader+(int(file[streamHeader])+1)*4]
	i := bytes.IndexByte(header[2:], 0x21) // the LZMA2 filter
	require.GreaterOrEqual(t, i, 0)
	header[2+i+2] = lzma.EncodeDictCap(dict)
	binary.LittleEndian.PutUint32(header[len(header)-4:], crc32.ChecksumIEEE(header[:len(header)-4]))
	return file
}

func openArchiveAt(t *testing.T, path string) any {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return openArchive(context.Background(), path, filepath.Base(path), info)
}

// A few bytes of .tar.xz asking for a 1 GiB dictionary made the decoder
// allocate 1 GiB, fatal on 32-bit builds. Such an archive is counted as the
// file it is.
func TestTarXzAskingForAHugeDictionaryIsNotBrowsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bomb.tar.xz")
	require.NoError(t, os.WriteFile(path, xzAskingFor(t, tarOf(t, 1), 1<<30), 0o600))

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	archive := openArchiveAt(t, path)
	runtime.ReadMemStats(&after)

	assert.Nil(t, archive)
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(64<<20))

	// within the limit, it is browsed
	path = filepath.Join(t.TempDir(), "fine.tar.xz")
	require.NoError(t, os.WriteFile(path, xzAskingFor(t, tarOf(t, 1), 8<<20), 0o600))
	assert.NotNil(t, openArchiveAt(t, path))
}

func TestArchiveOfTooManyEntriesIsNotBrowsed(t *testing.T) {
	orig := maxArchiveEntries
	maxArchiveEntries = 10
	t.Cleanup(func() { maxArchiveEntries = orig })
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "many.tar"), tarOf(t, 11), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "few.tar"), tarOf(t, 10), 0o600))
	var zipped bytes.Buffer
	zw := zip.NewWriter(&zipped)
	for i := range 11 {
		_, err := zw.Create(fmt.Sprintf("f%d", i))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "many.zip"), zipped.Bytes(), 0o600))

	assert.Nil(t, openArchiveAt(t, filepath.Join(dir, "many.tar")))
	assert.Nil(t, openArchiveAt(t, filepath.Join(dir, "many.zip")))
	assert.NotNil(t, openArchiveAt(t, filepath.Join(dir, "few.tar")))

	// counted as files by a scan
	tree := CreateAnalyzer().AnalyzeDir(context.Background(), dir, Options{ArchiveBrowsing: true})
	tree.UpdateStats(false)
	for _, name := range []string{"many.tar", "many.zip"} {
		entry := childNamed(t, tree, name)
		assert.False(t, entry.IsDir(), name)
		assert.Equal(t, usageOf(t, filepath.Join(dir, name)), entry.GetUsage(), name)
	}
	assert.True(t, childNamed(t, tree, "few.tar").IsDir())
}

// A stopped scan does not wait for a compressed tar to be decompressed.
func TestReadingATarStopsWithTheScan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.tar")
	require.NoError(t, os.WriteFile(path, tarOf(t, 1), 0o600))
	info, err := os.Stat(path)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = readTarFile(ctx, path, info)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestClampSize(t *testing.T) {
	assert.Equal(t, int64(5), clampSize(5))
	assert.Equal(t, int64(1<<63-1), clampSize(1<<64-1))
}
