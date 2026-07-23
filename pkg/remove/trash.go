//go:build !windows

package remove

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

type trashInfoFile interface {
	WriteString(string) (int, error)
	Close() error
}

type trashOSOps struct {
	mkdirAll        func(string, os.FileMode) error
	abs             func(string) (string, error)
	userHomeDir     func() (string, error)
	lstat           func(string) (os.FileInfo, error)
	openTrashInfo   func(string, int, os.FileMode) (trashInfoFile, error)
	remove          func(string) error
	rename          func(string, string) error
	removeAll       func(string) error
	mkdir           func(string, os.FileMode) error
	readDir         func(string) ([]os.DirEntry, error)
	readlink        func(string) (string, error)
	symlink         func(string, string) error
	openSource      func(string) (io.ReadCloser, error)
	openDestination func(string, int, os.FileMode) (io.WriteCloser, error)
	copy            func(io.Writer, io.Reader) (int64, error)
}

var trashOS = trashOSOps{
	mkdirAll:    os.MkdirAll,
	abs:         filepath.Abs,
	userHomeDir: os.UserHomeDir,
	lstat:       os.Lstat,
	openTrashInfo: func(name string, flag int, perm os.FileMode) (trashInfoFile, error) {
		return os.OpenFile(name, flag, perm)
	},
	remove:    os.Remove,
	rename:    renameNoReplace,
	removeAll: os.RemoveAll,
	mkdir:     os.Mkdir,
	readDir:   os.ReadDir,
	readlink:  os.Readlink,
	symlink:   os.Symlink,
	openSource: func(name string) (io.ReadCloser, error) {
		return os.Open(name)
	},
	openDestination: func(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
		return os.OpenFile(name, flag, perm)
	},
	copy: io.Copy,
}

// MoveItemToTrash moves item into the XDG trash and updates the in-memory dir tree.
func MoveItemToTrash(dir, item fs.Item) error {
	trashRoot, err := trashDir()
	if err != nil {
		return err
	}
	filesDir := filepath.Join(trashRoot, "files")
	infoDir := filepath.Join(trashRoot, "info")
	if err := trashOS.mkdirAll(filesDir, 0o700); err != nil {
		return err
	}
	if err := trashOS.mkdirAll(infoDir, 0o700); err != nil {
		return err
	}

	absSrc, err := trashOS.abs(item.GetPath())
	if err != nil {
		return err
	}

	for range 10001 {
		destName, infoPath, err := reserveTrashInfo(filesDir, infoDir, item.GetName(), absSrc)
		if err != nil {
			return err
		}
		destPath := filepath.Join(filesDir, destName)

		if err := movePath(absSrc, destPath); err != nil {
			_ = trashOS.remove(infoPath) //nolint:errcheck // Best-effort rollback.
			if os.IsExist(err) {
				// Destination appeared after reservation; pick another name.
				continue
			}
			return err
		}

		dir.RemoveFile(item)
		return nil
	}

	return fmt.Errorf("could not find unique trash name for %s", item.GetName())
}

func trashDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "Trash"), nil
	}
	home, err := trashOS.userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "Trash"), nil
}

func reserveTrashInfo(filesDir, infoDir, base, absSrc string) (name, infoPath string, err error) {
	for attempt := 0; attempt <= 10000; attempt++ {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s.%d", base, attempt+1)
		}

		destPath := filepath.Join(filesDir, candidate)
		if _, err := trashOS.lstat(destPath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", "", err
		}

		infoPath := filepath.Join(infoDir, candidate+".trashinfo")
		err := writeTrashInfo(infoPath, absSrc)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", "", err
		}

		// The exclusive trashinfo file reserves this name among compliant
		// trash implementations. Recheck files/ to avoid clobbering stale
		// entries that have no corresponding trashinfo file.
		if _, err := trashOS.lstat(destPath); os.IsNotExist(err) {
			return candidate, infoPath, nil
		} else if err != nil {
			_ = trashOS.remove(infoPath) //nolint:errcheck // Best-effort rollback.
			return "", "", err
		}

		_ = trashOS.remove(infoPath) //nolint:errcheck // Best-effort rollback.
	}

	return "", "", fmt.Errorf("could not find unique trash name for %s", base)
}

func writeTrashInfo(infoPath, absSrc string) error {
	content := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		escapeTrashPath(absSrc),
		time.Now().Format("2006-01-02T15:04:05"),
	)

	file, err := trashOS.openTrashInfo(infoPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}

	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = trashOS.remove(infoPath) //nolint:errcheck // Best-effort rollback.
		return err
	}
	if err := file.Close(); err != nil {
		_ = trashOS.remove(infoPath) //nolint:errcheck // Best-effort rollback.
		return err
	}
	return nil
}

func escapeTrashPath(p string) string {
	var b strings.Builder
	for _, r := range p {
		if r == '/' || r == '-' || r == '.' || r == '_' || r == '~' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			for _, by := range []byte(string(r)) {
				fmt.Fprintf(&b, "%%%02X", by)
			}
		}
	}
	return b.String()
}

func movePath(src, dst string) error {
	err := trashOS.rename(src, dst)
	if err == nil {
		return nil
	}
	if os.IsExist(err) || !isEXDEV(err) {
		return err
	}
	if err := copyRecursively(src, dst); err != nil {
		_ = trashOS.removeAll(dst) //nolint:errcheck // Best-effort rollback.
		return err
	}
	return trashOS.removeAll(src)
}

func copyRecursively(src, dst string) error {
	info, err := trashOS.lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := trashOS.mkdir(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := trashOS.readDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyRecursively(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := trashOS.readlink(src)
		if err != nil {
			return err
		}
		return trashOS.symlink(target, dst)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported file type %s: %s", info.Mode().Type(), src)
	}

	in, err := trashOS.openSource(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := trashOS.openDestination(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := trashOS.copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
