//go:build !windows && !plan9

package tui

import (
	"os"
	"os/signal"
	"syscall"
)

func stopProcess() error {
	// chan for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGCONT)
	defer signal.Stop(sigChan)

	err := syscall.Kill(syscall.Getpid(), syscall.SIGTSTP)
	// wait continue
	<-sigChan

	return err
}
