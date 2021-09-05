// +build !windows

package tui

import (
	"os"
)

func getShellBin() string {
	shellbin, ok := os.LookupEnv("SHELL")
	if !ok {
		shellbin = "/bin/bash"
	}
	return shellbin
}
