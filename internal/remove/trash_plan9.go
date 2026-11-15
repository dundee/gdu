package remove

import (
	"errors"
	"os"
)

// isEXDEV reports whether rename failed because it crossed directories,
// which Plan9 never allows and reports as os.ErrInvalid
func isEXDEV(err error) bool {
	return errors.Is(err, os.ErrInvalid)
}
