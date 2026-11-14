//go:build darwin || (freebsd && (386 || amd64 || arm || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (netbsd && amd64) || (openbsd && (amd64 || arm64)) || (windows && (386 || amd64 || arm64))

package sqlite

// The build constraint lists the targets modernc.org/sqlite builds for, and
// driver_other.go has its negation, so that each target gets exactly one.
import (
	// nolint:revive // Why: importing SQLite driver for side effects
	_ "modernc.org/sqlite"
)

// checkAvailable checks if the modernc SQLite driver is available
func checkAvailable() error {
	return nil
}
