package tui

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// messageTimeout is how long a transient status message stays in the header.
const messageTimeout = 2 * time.Second

// Operating system identifiers as reported by runtime.GOOS.
const (
	osDarwin  = "darwin"
	osWindows = "windows"
	osLinux   = "linux"
)

var errClipboardUnavailable = errors.New("no clipboard tool found")

// clipboardCommand returns the command and arguments used to write text to the
// system clipboard for the given operating system. lookPath reports which tools
// are installed and wayland selects the preferred Linux/BSD tool ordering. The
// returned bool is false when no supported tool is available, for example on a
// headless machine.
func clipboardCommand(
	goos string,
	wayland bool,
	lookPath func(string) (string, error),
) ([]string, bool) {
	type candidate struct {
		name string
		args []string
	}

	var candidates []candidate
	switch goos {
	case osDarwin:
		candidates = []candidate{{name: "pbcopy"}}
	case osWindows:
		candidates = []candidate{{name: "clip"}}
	default:
		xclip := candidate{name: "xclip", args: []string{"-selection", "clipboard"}}
		xsel := candidate{name: "xsel", args: []string{"--clipboard", "--input"}}
		wlCopy := candidate{name: "wl-copy"}
		if wayland {
			candidates = []candidate{wlCopy, xclip, xsel}
		} else {
			candidates = []candidate{xclip, xsel, wlCopy}
		}
	}

	for _, c := range candidates {
		if path, err := lookPath(c.name); err == nil {
			return append([]string{path}, c.args...), true
		}
	}
	return nil, false
}

func copyToClipboard(text string) error {
	argv, ok := clipboardCommand(runtime.GOOS, os.Getenv("WAYLAND_DISPLAY") != "", exec.LookPath)
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

// showMessage briefly replaces the header text with a status message and
// restores the previous text afterwards.
func (ui *UI) showMessage(message string) {
	previousText := ui.header.GetText(false)
	ui.header.SetText(message)

	go func() {
		time.Sleep(messageTimeout)
		ui.app.QueueUpdateDraw(func() {
			ui.header.Clear()
			ui.header.SetText(previousText)
		})
	}()
}
