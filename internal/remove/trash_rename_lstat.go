//go:build !windows && !darwin

package remove

import "os"

// renameAfterLstat renames oldpath to newpath unless newpath exists, which
// it checks first. Something could take newpath in between, but the trash
// has reserved the name already, by creating its .trashinfo exclusively.
func renameAfterLstat(oldpath, newpath string) error {
	if _, err := os.Lstat(newpath); err == nil {
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(oldpath, newpath)
}
