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

// MoveItemToTrash moves item into the XDG trash and updates the in-memory dir tree.
func MoveItemToTrash(dir, item fs.Item) error {
	trashRoot, err := trashDir()
	if err != nil {
		return err
	}
	filesDir := filepath.Join(trashRoot, "files")
	infoDir := filepath.Join(trashRoot, "info")
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(infoDir, 0o700); err != nil {
		return err
	}

	absSrc, err := filepath.Abs(item.GetPath())
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
			_ = os.Remove(infoPath)
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
	home, err := os.UserHomeDir()
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
		if _, err := os.Lstat(destPath); err == nil {
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
		if _, err := os.Lstat(destPath); os.IsNotExist(err) {
			return candidate, infoPath, nil
		} else if err != nil {
			_ = os.Remove(infoPath)
			return "", "", err
		}

		_ = os.Remove(infoPath)
	}

	return "", "", fmt.Errorf("could not find unique trash name for %s", base)
}

func writeTrashInfo(infoPath, absSrc string) error {
	content := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		escapeTrashPath(absSrc),
		time.Now().Format("2006-01-02T15:04:05"),
	)

	file, err := os.OpenFile(infoPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}

	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(infoPath)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(infoPath)
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
	err := renameNoReplace(src, dst)
	if err == nil {
		return nil
	}
	if os.IsExist(err) || !isEXDEV(err) {
		return err
	}
	if err := copyRecursively(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	return os.RemoveAll(src)
}

func copyRecursively(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.Mkdir(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
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
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported file type %s: %s", info.Mode().Type(), src)
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
