//go:build !windows && !linux && !darwin

package remove

func renameNoReplace(oldpath, newpath string) error {
	return renameAfterLstat(oldpath, newpath)
}
