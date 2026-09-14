package webui

import (
	"fmt"
	"path/filepath"
	"runtime"
)

func openPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("refusing to reveal non-absolute path %q", path)
	}
	name, args := revealCommand(runtime.GOOS, path)
	return runDetached(name, args...)
}

func revealCommand(goos, path string) (name string, args []string) {
	switch goos {
	case goosDarwin:
		return cmdOpen, []string{path}
	case goosWindows:
		return "explorer", []string{path}
	default:
		return cmdXDGOpen, []string{path}
	}
}
