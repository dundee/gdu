package selfupdate

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
)

// Restart runs the binary at path with args and exits with its exit code.
// Windows cannot replace a running process, so the new binary runs as a child
// sharing the console. It only returns if the binary cannot be started.
func Restart(path string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// the child handles Ctrl+C, the parent only waits for it
	signal.Ignore(os.Interrupt)

	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
