package webui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Platform identifiers and open-handler command names.
const (
	goosDarwin  = "darwin"
	goosWindows = "windows"
	cmdOpen     = "open"
	cmdXDGOpen  = "xdg-open"
)

// browserCommand builds the command used to open url. When cmd is non-empty it
// is treated as a launcher command line (the URL is appended as the final
// argument); otherwise the platform default handler for goos is used.
func browserCommand(goos, url, cmd string) (name string, args []string, err error) {
	if cmd != "" {
		fields := strings.Fields(cmd)
		if len(fields) == 0 {
			return "", nil, fmt.Errorf("empty browser command")
		}
		return fields[0], append(fields[1:], url), nil
	}

	switch goos {
	case goosDarwin:
		return cmdOpen, []string{url}, nil
	case goosWindows:
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}, nil
	default:
		return cmdXDGOpen, []string{url}, nil
	}
}

// openBrowser opens url in a browser using the configured or default launcher.
func openBrowser(url, cmd string) error {
	name, args, err := browserCommand(runtime.GOOS, url, cmd)
	if err != nil {
		return err
	}
	return runDetached(name, args...)
}

// runDetached starts an external command without waiting for its output,
// used for handing a URL off to the OS's URL handler. It still reaps the
// process asynchronously once it exits, so it doesn't accumulate unreaped
// processes for the server's lifetime.
func runDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait() //nolint:errcheck // best-effort reap; nothing to act on if it fails
	}()
	return nil
}
