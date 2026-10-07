//go:build !linux

package analyze

import "os"

// exclusiveUsage is only implemented on Linux (FIEMAP); elsewhere the
// st_blocks based usage is kept.
func exclusiveUsage(_ string, _ os.FileInfo, usage int64) (int64, bool) {
	return usage, false
}
