//go:build !windows

package selfupdate

import (
	"os"
	"syscall"
)

// Restart replaces the current process with the binary at path, run with args.
// It only returns on failure.
func Restart(path string, args []string) error {
	argv := append([]string{os.Args[0]}, args...)
	return syscall.Exec(path, argv, os.Environ())
}
