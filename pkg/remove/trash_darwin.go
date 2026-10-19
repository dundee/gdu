//go:build darwin

package remove

import (
	"fmt"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// MoveItemToTrash is not supported on macOS. The XDG trash location is ignored
// by Finder, so use permanent delete (d) or empty (e) instead.
func MoveItemToTrash(dir, item fs.Item) error {
	return fmt.Errorf("move to trash is not supported on macOS")
}
