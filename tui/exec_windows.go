package tui

import (
	"os"
	"os/exec"
)

func getShellBin() string {
	shellbin, ok := os.LookupEnv("COMSPEC")
	if !ok {
		shellbin = "C:\\WINDOWS\\System32\\cmd.exe"
	}
	return shellbin
}
