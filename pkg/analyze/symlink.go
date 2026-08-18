package analyze

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v5/pkg/annex"
)

// readSymlinkTarget returns the target path of the entry at path if it is a
// symlink, or an empty string otherwise (including when the link cannot be
// read). The mode is taken from the directory entry to avoid an extra stat.
func readSymlinkTarget(mode os.FileMode, path string) string {
	if mode&os.ModeSymlink == 0 {
		return ""
	}
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}

func followSymlink(path string, gitAnnexedSize bool) (tInfo os.FileInfo, err error) {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		target, err = os.Readlink(path)
		if err != nil {
			return nil, err
		}
		if gitAnnexedSize && strings.Contains(target, ".git/annex/objects") {
			tInfo, err = os.Lstat(path)
			if err != nil {
				return nil, err
			}

			name := filepath.Base(target)
			tInfo = annex.AnnexedFileInfo(tInfo, name)
			return tInfo, nil
		}
	}

	tInfo, err = os.Lstat(target)
	if err != nil {
		return nil, err
	}

	if tInfo.IsDir() {
		return nil, nil
	}

	return tInfo, nil
}
