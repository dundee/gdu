package analyze

import "github.com/dundee/gdu/v5/pkg/fs"

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
