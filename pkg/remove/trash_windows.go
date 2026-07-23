//go:build windows

package remove

import (
	"fmt"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// MoveItemToTrash moves item into the system trash.
func MoveItemToTrash(dir, item fs.Item) error {
	return fmt.Errorf("move to trash is not supported on Windows")
}
