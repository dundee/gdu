//go:build windows || plan9

package device

// GetFilesystemID is not supported on this platform
func GetFilesystemID(_ string) (uint64, bool) {
	return 0, false
}
