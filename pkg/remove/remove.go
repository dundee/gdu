package remove

import (
	"os"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// ItemFromDir removes item from dir
func ItemFromDir(dir, item fs.Item) error {
	err := os.RemoveAll(item.GetPath())
	if err != nil {
		return err
	}

	dir.RemoveFile(item)
	return nil
}

// EmptyFileFromDir empties file from dir (truncates to 0 bytes)
func EmptyFileFromDir(dir, file fs.Item) error {
	err := os.Truncate(file.GetPath(), 0)
	if err != nil {
		return err
	}

	// Remove old file and add zero-sized one
	dir.RemoveFile(file)
	newFile := &analyze.File{
		Name:   file.GetName(),
		Flag:   file.GetFlag(),
		Size:   0,
		Usage:  0,
		Parent: dir,
	}
	dir.AddFile(newFile)
	return nil
}
