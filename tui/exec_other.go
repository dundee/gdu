//go:build !windows

package tui

import (
	"os"
	"os/signal"
	"syscall"
)

func getShellBin() string {
	shellbin, ok := os.LookupEnv("SHELL")
	if !ok {
		shellbin = "/bin/bash"
	}
	return shellbin
}

func (ui *UI) spawnShell() {
	if ui.currentDir == nil {
		return
	}

	ui.app.Suspend(func() {
		if err := os.Chdir(ui.currentDirPath); err != nil {
			ui.showErr("Error changing directory", err)
			return
		}

		if err := ui.exec(getShellBin(), nil, os.Environ()); err != nil {
			ui.showErr("Error executing shell", err)
		}
	})
}

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
