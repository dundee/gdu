//go:build !(darwin || (freebsd && (386 || amd64 || arm || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (netbsd && amd64) || (openbsd && (amd64 || arm64)) || (windows && (386 || amd64 || arm64)))

package sqlite

import "errors"

// checkAvailable reports that the modernc SQLite driver is not available on this platform
func checkAvailable() error {
	return errors.New("modernc SQLite driver is not available on this platform")
}
