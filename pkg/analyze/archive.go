package analyze

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// IsArchiveDir reports whether item is a directory made up from the contents
// of a browsed zip or tar archive, including the node standing for the archive
// itself. Its children exist only inside the archive file.
func IsArchiveDir(item fs.Item) bool {
	switch dir := item.(type) {
	case *ZipDir, *TarDir:
		return true
	case *StoredDir:
		return dir.inArchive()
	}
	return false
}

// IsArchiveEntry reports whether item lives inside a browsed archive and so has
// no path of its own on the filesystem. The node standing for the archive
// itself is not an entry: its path is the real archive file.
func IsArchiveEntry(item fs.Item) bool {
	switch item.(type) {
	case *ZipFile, *TarFile:
		return true
	case nil:
		return false
	}
	return IsArchiveDir(item.GetParent())
}

// maxArchiveEntries is how many entries gdu reads of an archive at most;
// past it, the archive is counted as the file it is. The headers of a few
// kilobytes of gzip can make millions of entries.
var maxArchiveEntries = 1_000_000

// errTooManyEntries is returned for an archive of more than
// maxArchiveEntries entries.
var errTooManyEntries = errors.New("too many entries to browse")

// isPathElement reports whether name can only name an entry of its directory.
func isPathElement(name string) bool {
	switch name {
	case "", ".", "..":
		return false
	}
	return !strings.ContainsAny(name, "/\x00"+string(filepath.Separator)) && filepath.VolumeName(name) == ""
}

// archiveEntryPath returns the path of the entry name inside its archive, its
// names joined by "/". The author of the archive chose name: the parts which
// would climb out of the archive or start elsewhere ("..", a leading "/", a
// Windows volume) are dropped, as tar and unzip do when extracting, so that
// the path of every entry lies inside the archive. ok is false when no part
// is left.
func archiveEntryPath(name string) (path string, ok bool) {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == filepath.Separator })
	kept := parts[:0]
	for _, part := range parts {
		if isPathElement(part) {
			kept = append(kept, part)
		}
	}
	if len(kept) == 0 {
		return "", false
	}
	return strings.Join(kept, "/"), true
}
