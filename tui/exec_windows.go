package tui

import (
	"os"
	"os/exec"
)

func Execute(argv0 string, argv []string, envv []string) error {
	// Windows does not support exec syscall.
	cmd := exec.Command(argv0, argv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = envv
	err := cmd.Run()
	if err == nil {
		os.Exit(0)
	}
	return err
}
