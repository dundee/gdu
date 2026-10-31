//go:build !unix

package common

// getFilesystemID is not supported on this platform,
// filesystem boundaries are detected using mount points only
var getFilesystemID = func(_ string) (uint64, bool) {
	return 0, false
}
