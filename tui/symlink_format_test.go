package tui

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
)

func TestFormatSymlinkRow(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)

	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "test_dir",
			Usage: 100,
		},
	}

	symlink := &analyze.File{
		Name:    "bin",
		Parent:  dir,
		Usage:   4,
		Size:    7,
		Flag:    '@',
		Symlink: "usr/bin",
	}

	symlinkRow := ui.formatFileRow(symlink, 100, 100, false, false)

	if !bytes.Contains([]byte(symlinkRow), []byte("[aqua::b]")) {
		t.Error("symlink row should contain [aqua::b]")
	}
	if !bytes.Contains([]byte(symlinkRow), []byte("-> usr/bin")) {
		t.Error("symlink row should contain '-> usr/bin'")
	}
}

func TestSymlinkTargetInfoLine(t *testing.T) {
	symlink := &analyze.File{
		Name:    "bin",
		Flag:    '@',
		Symlink: "usr/bin",
	}
	line := symlinkTargetInfoLine(symlink)
	if !bytes.Contains([]byte(line), []byte("Target:")) {
		t.Errorf("info line should contain 'Target:', got %q", line)
	}
	if !bytes.Contains([]byte(line), []byte("usr/bin")) {
		t.Errorf("info line should contain 'usr/bin', got %q", line)
	}
}

func TestSymlinkTargetInfoLineForNonSymlink(t *testing.T) {
	regular := &analyze.File{Name: "file", Size: 5}
	if line := symlinkTargetInfoLine(regular); line != "" {
		t.Errorf("non-symlink should produce empty info line, got %q", line)
	}
}
