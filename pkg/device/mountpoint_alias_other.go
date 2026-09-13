//go:build !darwin

package device

func getMountPointAlias(_ string) (string, bool) {
	return "", false
}
