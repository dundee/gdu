package tui

import (
	"os"
	"os/exec"
)

// Execute runs given bin path via exec.Command call
func Execute(argv0 string, argv []string, envv []string) error {
	cmd := exec.Command(argv0, argv...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = envv

	return cmd.Run()
}
