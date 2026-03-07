//go:build (linux && !mips64 && !mipsle && !mips && !mips64le && !ppc64) || darwin || windows || (freebsd && !arm && !386) || (openbsd && !386) || (netbsd && !arm && !386 && !amd64)

package analyze

import (
	// nolint:revive // Why: importing SQLite driver for side effects
	_ "modernc.org/sqlite"
)

// checkAvailable checks if the modernc SQLite driver is available
func checkAvailable() error {
	return nil
}
