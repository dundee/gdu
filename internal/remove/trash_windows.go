//go:build windows

package remove

import (
	"fmt"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// MoveItemToTrash is not supported on Windows; use Unix XDG trash builds instead.
func MoveItemToTrash(fs.Item) (moved bool, err error) {
	return false, fmt.Errorf("move to trash is not supported on Windows")
}
