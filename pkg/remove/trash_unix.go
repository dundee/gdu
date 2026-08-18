//go:build !windows && !darwin

package remove

import (
	"errors"
	"syscall"
)

func isEXDEV(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
