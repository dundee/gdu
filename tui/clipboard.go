package tui

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/pkg/errors"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// Operating system identifiers as reported by runtime.GOOS.
const (
	osDarwin  = "darwin"
	osWindows = "windows"
)

var errClipboardUnavailable = errors.New("no clipboard tool found")

// clipboardTool is an external program able to write the clipboard, together
// with the arguments it needs to target the clipboard selection.
type clipboardTool struct {
	name string
	args []string
}

var (
	pbcopy = clipboardTool{name: "pbcopy"}
	clip   = clipboardTool{name: "clip"}
	xclip  = clipboardTool{name: "xclip", args: []string{"-selection", "clipboard"}}
	xsel   = clipboardTool{name: "xsel", args: []string{"--clipboard", "--input"}}
	wlCopy = clipboardTool{name: "wl-copy"}
)

// clipboardEnv is the part of the environment that decides which clipboard tool
// gdu uses. It exists so the choice can be tested without a real desktop
// session: lookPath reports which tools are installed, and wayland selects the
// preferred Linux/BSD tool ordering.
type clipboardEnv struct {
	goos     string
	wayland  bool
	lookPath func(string) (string, error)
}

// currentClipboardEnv describes the environment gdu is actually running in.
func currentClipboardEnv() clipboardEnv {
	return clipboardEnv{
		goos:     runtime.GOOS,
		wayland:  os.Getenv("WAYLAND_DISPLAY") != "",
		lookPath: exec.LookPath,
	}
}

// command returns the command and arguments used to write text to the system
// clipboard in this environment. The returned bool is false when no supported
// tool is available, for example on a headless machine.
func (env clipboardEnv) command() ([]string, bool) {
	var candidates []clipboardTool
	switch env.goos {
	case osDarwin:
		candidates = []clipboardTool{pbcopy}
	case osWindows:
		candidates = []clipboardTool{clip}
	default:
		if env.wayland {
			candidates = []clipboardTool{wlCopy, xclip, xsel}
		} else {
			candidates = []clipboardTool{xclip, xsel, wlCopy}
		}
	}

	for _, c := range candidates {
		if path, err := env.lookPath(c.name); err == nil {
			return append([]string{path}, c.args...), true
		}
	}
	return nil, false
}

func copyToClipboard(text string) error {
	argv, ok := currentClipboardEnv().command()
	if !ok {
		return errClipboardUnavailable
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "running clipboard command")
	}
	return nil
}

// copySelectedPath copies the path of the item under the cursor to the system
// clipboard.
func (ui *UI) copySelectedPath() {
	if ui.currentDir == nil {
		return
	}

	row, column := ui.table.GetSelection()
	cell := ui.table.GetCell(row, column)
	if cell == nil || cell.GetReference() == nil {
		return
	}
	selectedItem := cell.GetReference().(fs.Item)

	if err := copyToClipboard(selectedItem.GetPath()); err != nil {
		ui.showErr("Can't copy path to clipboard", err)
		return
	}

	ui.showMessage(" Path copied to clipboard")
}
