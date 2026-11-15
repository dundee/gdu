//go:build darwin

package remove

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// macOSTrashBin is the trash CLI shipped with macOS. It moves items into the
// Finder trash, so they can be put back from there.
var macOSTrashBin = "/usr/bin/trash"

// MoveItemToTrash moves the item into the Finder trash using the system trash
// command. The path is passed as an argument without a shell; it is absolute,
// so it can never be taken for an option. The item counts as moved only when
// the path is really gone, as for TrashCommand.
func MoveItemToTrash(item fs.Item) (moved bool, err error) {
	if _, err := os.Stat(macOSTrashBin); errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("%s not found; set trash-command to a trash CLI instead", macOSTrashBin)
	}

	absPath, err := trashCommandOS.abs(item.GetPath())
	if err != nil {
		return false, err
	}

	var stderr bytes.Buffer
	if err := trashCommandOS.run(macOSTrashBin, []string{absPath}, trashCommandOS.environ(), &stderr); err != nil {
		return false, commandError(macOSTrashBin, err, stderr.String())
	}
	_, err = trashCommandOS.lstat(absPath)
	return err != nil, nil
}
