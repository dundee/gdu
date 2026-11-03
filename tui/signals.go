//go:build !plan9

package tui

import (
	"os"
	"syscall"
)

var handledSignals = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGQUIT,
	syscall.SIGILL,
	syscall.SIGTRAP,
	syscall.SIGABRT,
	syscall.SIGPIPE,
	syscall.SIGTERM,
}
