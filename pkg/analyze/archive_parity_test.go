package analyze

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// archiveContent is what the test archives hold: compressible data, so that
// it is far larger than the archives, in a directory below another.
var archiveContent = map[string]int{"inner/big": 2 << 20, "inner/deeper/x": 1000, "top": 10}

var longAgo = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

func writeArchiveEntries(t *testing.T, add func(name string, data []byte)) {
	t.Helper()
	for name, size := range archiveContent {
		add(name, make([]byte, size))
	}
}

// makeArchiveTree creates a tree holding a .tar.gz and a .zip, all of it
// last modified long ago.
func makeArchiveTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o600))

	out, err := os.Create(filepath.Join(sub, "data.tar.gz"))
	require.NoError(t, err)
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	writeArchiveEntries(t, func(name string, data []byte) {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg, Name: name, Size: int64(len(data)), Mode: 0o644, ModTime: longAgo,
		}))
		_, err := tw.Write(data)
		require.NoError(t, err)
	})
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	require.NoError(t, out.Close())

	out, err = os.Create(filepath.Join(sub, "data.zip"))
	require.NoError(t, err)
	zw := zip.NewWriter(out)
	writeArchiveEntries(t, func(name string, data []byte) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: longAgo})
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	})
	require.NoError(t, zw.Close())
	require.NoError(t, out.Close())

	for _, p := range []string{"a.txt", "sub/data.tar.gz", "sub/data.zip", "sub", "."} {
		require.NoError(t, os.Chtimes(filepath.Join(root, p), longAgo, longAgo))
	}
	return root
}

func childNamed(t *testing.T, dir fs.Item, name string) fs.Item {
	t.Helper()
	for entry := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		if entry.GetName() == name {
			return entry
		}
	}
	t.Fatalf("%s has no entry %s", dir.GetPath(), name)
	return nil
}

func scanArchives(t *testing.T, a Analyzer, root string, browse bool) fs.Item {
	t.Helper()
	tree := a.AnalyzeDir(context.Background(), root, Options{ArchiveBrowsing: browse})
	tree.UpdateStats(false)
	return tree
}

// Browsing an archive shows what it holds, uncompressed, but the archive
// counts as the file it is: every analyzer reports the totals it reports
// without archive browsing, and no directory gets the time of the scan for
// its mtime.
func TestArchiveBrowsingChangesNoTotals(t *testing.T) {
	root := makeArchiveTree(t)
	plain := scanArchives(t, CreateAnalyzer(), root, false)
	topDir := scanArchives(t, CreateTopDirAnalyzer(), root, true)
	require.Equal(t, plain.GetUsage(), topDir.GetUsage(), "the top-dir analyzer does not browse archives")

	analyzers := append([]parityAnalyzer{{
		name:     "parallel",
		analyzer: func(*testing.T) Analyzer { return CreateAnalyzer() },
	}}, parityAnalyzers()...)
	for _, tc := range analyzers {
		t.Run(tc.name, func(t *testing.T) {
			tree := scanArchives(t, tc.analyzer(t), root, true)

			assert.Equal(t, plain.GetUsage(), tree.GetUsage())
			assert.Equal(t, plain.GetSize(), tree.GetSize())
			assert.Equal(t, longAgo, tree.GetMtime().UTC(), "the root's mtime")
			sub := childNamed(t, tree, "sub")
			assert.Equal(t, plain.GetUsage()-usageOf(t, filepath.Join(root, "a.txt"))-usageOf(t, root), sub.GetUsage())
			assert.Equal(t, longAgo, sub.GetMtime().UTC(), "sub's mtime")

			for _, name := range []string{"data.tar.gz", "data.zip"} {
				archivePath := filepath.Join(root, "sub", name)
				info, err := os.Stat(archivePath)
				require.NoError(t, err)
				archive := childNamed(t, sub, name)

				assert.Equal(t, info.Size(), archive.GetSize(), name)
				assert.Equal(t, usageOf(t, archivePath), archive.GetUsage(), name)
				assert.Equal(t, longAgo, archive.GetMtime().UTC(), name)
				// the archive and its entries: inner, inner/big, inner/deeper,
				// inner/deeper/x and top
				assert.Equal(t, int64(6), archive.GetItemCount(), name)

				inner := childNamed(t, archive, "inner")
				assert.Equal(t, int64(2<<20+1000), inner.GetSize(), "%s shows its content uncompressed", name)
				assert.Equal(t, int64(2<<20), childNamed(t, inner, "big").GetSize(), name)
				assert.Equal(t, longAgo, inner.GetMtime().UTC(), name)
			}
		})
	}
}
