//go:build !darwin

package fs

// PreventDatalessMaterialization is a no-op on platforms other than macOS.
func PreventDatalessMaterialization() error {
	return nil
}
