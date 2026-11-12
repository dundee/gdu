//go:build !linux && !darwin

package analyze

import "os"

// extentMeter is only implemented on Linux (FIEMAP) and macOS (getattrlist);
// elsewhere the st_blocks based usage is kept and nothing is reported as
// shared.
type extentMeter struct{}

func (*extentMeter) measure(_ string, _ os.FileInfo, usage int64) (exclusive, shared int64) {
	return usage, 0
}
