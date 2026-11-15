//go:build windows

package remove

import (
	"fmt"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// TrashCommand is not supported on Windows, which has no POSIX shell to
// evaluate the command in.
func TrashCommand(command string) Trash {
	return func(fs.Item) (bool, error) {
		return false, fmt.Errorf("trash command is not supported on Windows")
	}
}
