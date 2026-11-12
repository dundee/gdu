package remove

import (
	"os"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// ItemFromDir removes item from dir
//
// Deprecated: Use Deleter, which refuses items outside the scanned roots
// before touching the disk.
func ItemFromDir(dir, item fs.Item) error {
	err := os.RemoveAll(item.GetPath())
	if err != nil {
		return err
	}

	dir.RemoveFile(item)
	return nil
}

// EmptyFileFromDir empties file from dir (truncates to 0 bytes)
//
// Deprecated: Use Deleter, which refuses items outside the scanned roots and
// never truncates the target of a link.
func EmptyFileFromDir(dir, file fs.Item) error {
	err := os.Truncate(file.GetPath(), 0)
	if err != nil {
		return err
	}

	replaceWithEmptyFile(dir, file)
	return nil
}

// replaceWithEmptyFile updates the tree after file was truncated to 0 bytes.
func replaceWithEmptyFile(dir, file fs.Item) {
	dir.RemoveFile(file)
	newFile := &analyze.File{
		Name:   file.GetName(),
		Flag:   file.GetFlag(),
		Size:   0,
		Usage:  0,
		Parent: dir,
	}
	dir.AddFile(newFile)
}
