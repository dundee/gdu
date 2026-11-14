//go:build linux

package remove

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// renameat2 is unix.Renameat2, replaced by tests.
var renameat2 = unix.Renameat2

func renameNoReplace(oldpath, newpath string) error {
	err := renameat2(unix.AT_FDCWD, oldpath, unix.AT_FDCWD, newpath, unix.RENAME_NOREPLACE)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.EEXIST):
		return os.ErrExist
	case errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EOPNOTSUPP):
		// The NFS client, FUSE servers without rename2 and older ZFS refuse
		// any flag with EINVAL, and kernels or seccomp profiles without
		// renameat2 fail with ENOSYS: the plain rename works there.
		return renameAfterLstat(oldpath, newpath)
	}
	return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: err}
}
