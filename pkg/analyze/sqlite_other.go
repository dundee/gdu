//go:build (linux && (mips64 || mipsle || mips || mips64le || ppc64)) || (freebsd && (arm || 386)) || (openbsd && 386) || (netbsd && (arm || 386 || amd64))

package analyze

import "errors"

// checkAvailable reports that the modernc SQLite driver is not available on this platform
func checkAvailable() error {
	return errors.New("modernc SQLite driver is not available on this platform")
}
