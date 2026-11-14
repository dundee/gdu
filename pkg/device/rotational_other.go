//go:build !linux

package device

// IsRotational reports whether path is on a rotational block device. Rotational
// detection is currently implemented only on Linux, so other platforms report
// it as unknown.
func IsRotational(path string) (rotational bool, known bool) {
	return false, false
}
