package tui

import (
	"os"
	"os/exec"
)

// Execute runs given bin path via exec.Command call
func Execute(argv0 string, argv, envv []string) error {
	cmd := exec.Command(argv0, argv...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = envv

	return cmd.Run()
}

func (ui *UI) getShellBin() string {
	if ui.shellBin != "" {
		return ui.shellBin
	}
	return defaultShellBin()
}

func (ui *UI) spawnShell() {
	if ui.currentDir == nil {
		return
	}
	if ui.atVirtualRoot() {
		ui.showMessage("Shell cannot be spawned at the virtual top level directory")
		return
	}

	ui.app.Suspend(func() {
		if err := os.Chdir(ui.currentDirPath); err != nil {
			ui.showErr("Error changing directory", err)
			return
		}

		if err := ui.exec(ui.getShellBin(), nil, os.Environ()); err != nil {
			ui.showErr("Error executing shell", err)
		}
	})
}
