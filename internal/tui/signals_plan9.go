package tui

import (
	"os"
	"syscall"
)

// Plan9 delivers notes instead of signals and defines only these
var handledSignals = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGABRT,
	syscall.SIGTERM,
}
