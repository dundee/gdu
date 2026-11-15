package webui

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// revealInFileManager opens path in the OS file manager.
//
// The path must be absolute. Live scans always produce absolute paths, but
// ReadAnalysis takes the root path verbatim from a JSON report, which is
// attacker-controlled input when the report came from somewhere else: a root
// named "-a" or "--new-window" would otherwise be handed to open/xdg-open as
// its first argument and read as a flag rather than a path. Requiring an
// absolute path rejects every such name. ("--" is not a portable guard here,
// since explorer does not honour it.)
func revealInFileManager(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("refusing to reveal non-absolute path %q", path)
	}
	name, args := revealCommand(runtime.GOOS, path)
	return runDetached(name, args...)
}

// revealCommand builds the command that opens path in the platform's file
// manager. Windows uses explorer rather than the URL handler browser.go
// reaches for, so the two platform switches stay separate.
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
