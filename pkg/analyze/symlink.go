package analyze

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v5/pkg/annex"
)

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
