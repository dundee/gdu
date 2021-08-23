// +build !windows

package tui

import (
	"os"
	"syscall"
)

// Execute runs given bin path via the Exec syscall
func Execute(argv0 string, argv []string, envv []string) error {
	// append argv0 to argv, as execve will make first argument the "binary name".
	return syscall.Exec(argv0, append([]string{argv0}, argv...), envv)
}

func getShellBin() string {
	shellbin, ok := os.LookupEnv("SHELL")
	if !ok {
		shellbin = "/bin/bash"
	}
	return shellbin
}
