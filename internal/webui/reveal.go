package webui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// revealInFileManager shows the item at path in the OS file manager, never
// opening it.
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
	info, err := os.Stat(path)
	name, args := revealCommand(runtime.GOOS, path, err == nil && info.IsDir())
	return runDetached(name, args...)
}

// revealCommand builds the command that shows the item at path, a directory
// when isDir, in the platform's file manager. Windows uses explorer rather
// than the URL handler browser.go reaches for, so the two platform switches
// stay separate.
//
// Opening the item would do what a double-click does: open an archive with
// its application, launch an .app bundle on macOS, or install a .prefPane. So
// Finder and Explorer select it in its directory instead. xdg-open can only
// open, so it opens the directory, or the one holding any other item.
func revealCommand(goos, path string, isDir bool) (name string, args []string) {
	switch goos {
	case goosDarwin:
		return cmdOpen, []string{"-R", path}
	case goosWindows:
		// explorer reads "/select," and the path as separate arguments
		return "explorer", []string{"/select,", path}
	default:
		if !isDir {
			path = filepath.Dir(path)
		}
		return cmdXDGOpen, []string{path}
	}
}
