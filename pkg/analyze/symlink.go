package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v5/pkg/annex"
)

func followSymlink(path string, gitAnnexedSize bool) (tInfo os.FileInfo, err error) {
	target, err := os.Readlink(path)
	if err != nil {
		fmt.Println(path, err)
		return nil, err
	}

	tInfo, err = os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) && gitAnnexedSize && strings.Contains(target, ".git/annex/objects") {
			tInfo, err = os.Lstat(path)
			if err != nil {
				return nil, err
			}

			name := filepath.Base(target)
			tInfo = annex.AnnexedFileInfo(tInfo, name)
		} else {
			return nil, err
		}
	}

	if tInfo.IsDir() {
		return nil, nil
	}

	return tInfo, nil
}
