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

// EmptyFileFromDir empty file from dir
func EmptyFileFromDir(dir, file fs.Item) error {
	err := os.Truncate(file.GetPath(), 0)
	if err != nil {
		return err
	}

	cur := dir.(*analyze.Dir)
	for {
		cur.Size -= file.GetSize()
		cur.Usage -= file.GetUsage()

		if cur.Parent == nil {
			break
		}
		cur = cur.Parent.(*analyze.Dir)
	}

	// Remove the old file and add a new one with zero size
	cur = dir.(*analyze.Dir)
	cur.Files = cur.Files.Remove(file)
	newFile := &analyze.File{
		Name:   file.GetName(),
		Flag:   file.GetFlag(),
		Size:   0,
		Parent: dir,
	}
	dir.AddFile(newFile)

	return nil
}
