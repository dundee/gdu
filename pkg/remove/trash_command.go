package remove

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// runTrashCommand runs the configured trash command. It is a package variable so
// tests can stub out the external process.
var runTrashCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// TrashByCommand returns a trasher that hands the item to an external trash tool
// instead of the built-in XDG trash. The command is given as the program name
// plus optional arguments (for example "trash-put", or "gio" "trash") and the
// item path is appended as the last argument. This lets gdu defer to a user's
// preferred trash tool and trash directory, and makes "move to trash" work on
// platforms where the built-in XDG trash does not apply.
func TrashByCommand(command []string) func(fs.Item, fs.Item) error {
	return func(dir, item fs.Item) error {
		if len(command) == 0 {
			return fmt.Errorf("no trash command configured")
		}

		args := make([]string, 0, len(command))
		args = append(args, command[1:]...)
		args = append(args, item.GetPath())

		if out, err := runTrashCommand(command[0], args...); err != nil {
			if msg := strings.TrimSpace(string(out)); msg != "" {
				return fmt.Errorf("%w: %s", err, msg)
			}
			return err
		}

		dir.RemoveFile(item)
		return nil
	}
}
