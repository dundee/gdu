//go:build darwin || (freebsd && (386 || amd64 || arm || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (netbsd && amd64) || (openbsd && (amd64 || arm64)) || (windows && (386 || amd64 || arm64))

package analyze

import (
	// nolint:revive // Why: importing SQLite driver for side effects
	_ "modernc.org/sqlite"
)
