//go:build darwin

package remove

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// macOSTrashBin is the trash CLI shipped with macOS. It moves items into the
// Finder trash, so they can be put back from there.
var macOSTrashBin = "/usr/bin/trash"

// MoveItemToTrash moves the item into the Finder trash using the system trash
// command. The path is passed as an argument without a shell; it is absolute,
// so it can never be taken for an option. The tree is updated only when the
// path is really gone, as for TrashCommand.
func MoveItemToTrash(dir, item fs.Item) error {
	if _, err := os.Stat(macOSTrashBin); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s not found; set trash-command to a trash CLI instead", macOSTrashBin)
	}

	absPath, err := trashCommandOS.abs(item.GetPath())
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	if err := trashCommandOS.run(macOSTrashBin, []string{absPath}, trashCommandOS.environ(), &stderr); err != nil {
		return commandError(macOSTrashBin, err, stderr.String())
	}
	if _, err := trashCommandOS.lstat(absPath); err == nil {
		return nil
	}

	dir.RemoveFile(item)
	return nil
}
