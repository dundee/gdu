//go:build (linux || darwin) && (amd64 || arm64 || loong64 || ppc64le || s390x || riscv64 || 386 || arm)

package analyze

import (
	// nolint:revive // use driver
	_ "modernc.org/sqlite"
)
