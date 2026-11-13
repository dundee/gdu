//go:build !windows && !darwin && !plan9

package remove

import (
	"errors"
	"syscall"
)

func isEXDEV(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
