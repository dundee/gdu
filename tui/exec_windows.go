package tui

import (
	"os"
)

func defaultShellBin() string {
	shellbin, ok := os.LookupEnv("COMSPEC")
	if !ok {
		shellbin = "C:\\WINDOWS\\System32\\cmd.exe"
	}
	return shellbin
}

func stopProcess() error {
	return nil
}
