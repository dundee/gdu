package tui

import (
	"bytes"
	"errors"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v5/internal/testapp"
)

func lookPathFor(names ...string) func(string) (string, error) {
	available := make(map[string]bool, len(names))
	for _, name := range names {
		available[name] = true
	}
	return func(name string) (string, error) {
		if available[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("executable not found")
	}
}

func TestClipboardCommand(t *testing.T) {
	for name, tt := range map[string]struct {
		env    clipboardEnv
		want   []string
		wantOK bool
	}{
		"darwin uses pbcopy": {
			env:  clipboardEnv{goos: osDarwin, lookPath: lookPathFor("pbcopy")},
			want: []string{"/usr/bin/pbcopy"}, wantOK: true,
		},
		"windows uses clip": {
			env:  clipboardEnv{goos: osWindows, lookPath: lookPathFor("clip")},
			want: []string{"/usr/bin/clip"}, wantOK: true,
		},
		"linux uses xclip": {
			env:  clipboardEnv{goos: "linux", lookPath: lookPathFor("xclip", "xsel")},
			want: []string{"/usr/bin/xclip", "-selection", "clipboard"}, wantOK: true,
		},
		"linux falls back to xsel": {
			env:  clipboardEnv{goos: "linux", lookPath: lookPathFor("xsel")},
			want: []string{"/usr/bin/xsel", "--clipboard", "--input"}, wantOK: true,
		},
		"freebsd uses the same tools as linux": {
			env:  clipboardEnv{goos: "freebsd", lookPath: lookPathFor("xclip")},
			want: []string{"/usr/bin/xclip", "-selection", "clipboard"}, wantOK: true,
		},
		"wayland session prefers wl-copy": {
			env:  clipboardEnv{goos: "linux", wayland: true, lookPath: lookPathFor("wl-copy", "xclip")},
			want: []string{"/usr/bin/wl-copy"}, wantOK: true,
		},
		"without wayland session prefers xclip over wl-copy": {
			env:  clipboardEnv{goos: "linux", wayland: false, lookPath: lookPathFor("wl-copy", "xclip")},
			want: []string{"/usr/bin/xclip", "-selection", "clipboard"}, wantOK: true,
		},
		"falls back to wl-copy when only tool": {
			env:  clipboardEnv{goos: "linux", wayland: false, lookPath: lookPathFor("wl-copy")},
			want: []string{"/usr/bin/wl-copy"}, wantOK: true,
		},
		"no tool available": {
			env:  clipboardEnv{goos: "linux", lookPath: lookPathFor()},
			want: nil, wantOK: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := tt.env.command()
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCopyToClipboardRoundTrip(t *testing.T) {
	if runtime.GOOS != osDarwin {
		t.Skip("clipboard round-trip is only verified with pbcopy/pbpaste on darwin")
	}
	if _, err := exec.LookPath("pbcopy"); err != nil {
		t.Skip("pbcopy not available")
	}

	// Preserve and restore whatever the user already had on the clipboard.
	before, _ := exec.Command("pbpaste").Output()
	t.Cleanup(func() {
		restore := exec.Command("pbcopy")
		restore.Stdin = bytes.NewReader(before)
		_ = restore.Run()
	})

	const path = "/tmp/gdu clipboard test/some file"
	if err := copyToClipboard(path); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, path, string(out))
}

func TestCopySelectedPathWithoutCurrentDir(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	// No directory analyzed yet: copying must be a safe no-op.
	assert.NotPanics(t, ui.copySelectedPath)
}
