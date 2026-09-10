package webui

import "runtime"

func openPath(path string) error {
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
