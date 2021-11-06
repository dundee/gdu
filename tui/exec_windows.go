package tui

import (
	"os"
)

func getShellBin() string {
	shellbin, ok := os.LookupEnv("COMSPEC")
	if !ok {
		shellbin = "C:\\WINDOWS\\System32\\cmd.exe"
	}
	return shellbin
}

func (ui *UI) spawnShell() {
	if ui.currentDir == nil {
		return
	}

	ui.app.Stop()

	if err := os.Chdir(ui.currentDirPath); err != nil {
		ui.showErr("Error changing directory", err)
		return
	}
	if err := ui.exec(getShellBin(), nil, os.Environ()); err != nil {
		ui.showErr("Error executing shell", err)
	}
}
