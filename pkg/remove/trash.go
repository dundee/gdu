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

	base := item.GetName()
	destName, err := uniqueTrashName(filesDir, infoDir, base)
	if err != nil {
		return err
	}
	destPath := filepath.Join(filesDir, destName)
	infoPath := filepath.Join(infoDir, destName+".trashinfo")

	absSrc, err := filepath.Abs(item.GetPath())
	if err != nil {
		return err
	}

	if err := writeTrashInfo(infoPath, absSrc); err != nil {
		return err
	}
	if err := movePath(absSrc, destPath); err != nil {
		_ = os.Remove(infoPath)
		return err
	}

	dir.RemoveFile(item)
	return nil
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

func uniqueTrashName(filesDir, infoDir, base string) (string, error) {
	candidate := base
	for i := 1; ; i++ {
		_, errF := os.Stat(filepath.Join(filesDir, candidate))
		_, errI := os.Stat(filepath.Join(infoDir, candidate+".trashinfo"))
		if os.IsNotExist(errF) && os.IsNotExist(errI) {
			return candidate, nil
		}
		if i == 1 {
			candidate = base + ".2"
		} else {
			candidate = fmt.Sprintf("%s.%d", base, i+1)
		}
		if i > 10000 {
			return "", fmt.Errorf("could not find unique trash name for %s", base)
		}
	}
}

func writeTrashInfo(infoPath, absSrc string) error {
	content := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		escapeTrashPath(absSrc),
		time.Now().Format("2006-01-02T15:04:05"),
	)
	return os.WriteFile(infoPath, []byte(content), 0o600)
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
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !isEXDEV(err) {
		return err
	}
	if err := copyRecursively(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	return os.RemoveAll(src)
}

func copyRecursively(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
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
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
