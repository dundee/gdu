//go:build !linux

package analyze

import "os"

// measureExclusiveUsage is only implemented on Linux (FIEMAP); elsewhere the
// st_blocks based usage is kept and nothing is reported as shared.
func measureExclusiveUsage(_ string, _ os.FileInfo, usage int64) (exclusive, shared int64) {
	return usage, 0
}
