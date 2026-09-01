package webui

import "runtime"

func openPath(path string) error {
	name, args := revealCommand(path)
	return runDetached(name, args...)
}

func revealCommand(path string) (name string, args []string) {
	switch runtime.GOOS {
	case goosDarwin:
		return cmdOpen, []string{path}
	case goosWindows:
		return "explorer", []string{path}
	default:
		return cmdXDGOpen, []string{path}
	}
}
