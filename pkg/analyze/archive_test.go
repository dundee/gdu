package analyze

import (
	"archive/tar"
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// hostileEntryNames are archive entry names climbing out of the archive or
// starting elsewhere, with what is left of them inside it.
var hostileEntryNames = map[string]string{
	"../victim/f":     "victim/f",
	"a/../../b":       "a/b",
	"/abs/c":          "abs/c",
	"./d":             "d",
	"e//f":            "e/f",
	"ok/g":            "ok/g",
	"../../..":        "",
	"..":              "",
	"x/./../y/../z/h": "x/y/z/h",
}

func TestArchiveEntryPath(t *testing.T) {
	for name, want := range hostileEntryNames {
		got, ok := archiveEntryPath(name)

		assert.Equal(t, want != "", ok, name)
		assert.Equal(t, want, got, name)
	}
}

func writeHostileZip(t *testing.T, archivePath string) {
	t.Helper()
	out, err := os.Create(archivePath)
	require.NoError(t, err)
	w := zip.NewWriter(out)
	for name := range hostileEntryNames {
		entry, err := w.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte("x"))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())
}

func writeHostileTar(t *testing.T, archivePath string) {
	t.Helper()
	out, err := os.Create(archivePath)
	require.NoError(t, err)
	w := tar.NewWriter(out)
	require.NoError(t, w.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "../up/", Mode: 0o755}))
	for name := range hostileEntryNames {
		require.NoError(t, w.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: 1, Mode: 0o644}))
		_, err := w.Write([]byte("x"))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, out.Close())
}

// assertInsideArchive asserts that every entry below dir is named by a single
// name and has a path below the archive's.
func assertInsideArchive(t *testing.T, archivePath string, dir fs.Item) {
	t.Helper()
	for entry := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		assert.True(t, isPathElement(entry.GetName()), "%q is no single name", entry.GetName())
		path := filepath.ToSlash(entry.GetPath())
		assert.True(t, strings.HasPrefix(path, filepath.ToSlash(archivePath)+"/"), "%s lies outside %s", path, archivePath)
		if entry.IsDir() {
			assertInsideArchive(t, archivePath, entry)
		}
	}
}

// entryPaths returns the paths of the files below dir, relative to it.
func entryPaths(dir fs.Item, prefix string, paths map[string]bool) map[string]bool {
	for entry := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
		if entry.IsDir() {
			entryPaths(entry, prefix+entry.GetName()+"/", paths)
			continue
		}
		paths[prefix+entry.GetName()] = true
	}
	return paths
}

// An archive entry such as "../victim/f" used to become a directory named
// "..", whose path was the directory holding the archive. Every entry now
// lies inside the archive, under what is left of its name.
func TestArchiveEntriesStayInsideTheArchive(t *testing.T) {
	want := map[string]bool{}
	for _, inside := range hostileEntryNames {
		if inside != "" {
			want[inside] = true
		}
	}

	for _, kind := range []string{"zip", "tar"} {
		t.Run(kind, func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "scan", "evil."+kind)
			require.NoError(t, os.MkdirAll(filepath.Dir(archivePath), 0o755))
			info := func() os.FileInfo {
				stat, err := os.Stat(archivePath)
				require.NoError(t, err)
				return stat
			}

			var archive fs.Item
			if kind == "zip" {
				writeHostileZip(t, archivePath)
				dir, err := processZipFile(archivePath, info())
				require.NoError(t, err)
				archive = dir
			} else {
				writeHostileTar(t, archivePath)
				dir, err := processTarFile(archivePath, info())
				require.NoError(t, err)
				archive = dir
			}

			assertInsideArchive(t, archivePath, archive)
			got := entryPaths(archive, "", map[string]bool{})
			assert.Equal(t, want, got)
		})
	}
}
