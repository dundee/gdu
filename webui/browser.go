package webui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
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
	case "darwin":
		return "open", []string{url}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}, nil
	default:
		return "xdg-open", []string{url}, nil
	}
}

// openBrowser opens url in a browser using the configured or default launcher.
func openBrowser(url, cmd string) error {
	name, args, err := browserCommand(runtime.GOOS, url, cmd)
	if err != nil {
		return err
	}
	return exec.Command(name, args...).Start()
}
