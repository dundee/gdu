//go:build !linux

package device

func getFilesystemID(_ string) (uint64, bool) {
	return 0, false
}
