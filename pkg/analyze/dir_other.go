//go:build windows || plan9
// +build windows plan9

package analyze

import (
	"os"
)

func setPlatformSpecificAttrs(file *File, f os.FileInfo) {}

func setDirPlatformSpecificAttrs(dir *Dir, path string) {}
