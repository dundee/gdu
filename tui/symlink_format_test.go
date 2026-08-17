package tui

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
)

func newSymlinkTestFile() *analyze.File {
	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "test_dir",
			Usage: 100,
		},
	}
	return &analyze.File{
		Name:    "bin",
		Parent:  dir,
		Usage:   4,
		Size:    7,
		Flag:    '@',
		Symlink: "usr/bin",
	}
}

func TestFormatSymlinkRow(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)
	ui.SetShowSymlinkTarget(true)

	symlinkRow := ui.formatFileRow(newSymlinkTestFile(), 100, 100, false, false)

	if !bytes.Contains([]byte(symlinkRow), []byte("[aqua::b]")) {
		t.Error("symlink row should contain [aqua::b]")
	}
	if !bytes.Contains([]byte(symlinkRow), []byte("-> usr/bin")) {
		t.Error("symlink row should contain '-> usr/bin'")
	}
}

func TestFormatSymlinkRowDisabledByDefault(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)

	symlinkRow := ui.formatFileRow(newSymlinkTestFile(), 100, 100, false, false)

	if bytes.Contains([]byte(symlinkRow), []byte("-> usr/bin")) {
		t.Error("symlink target should not be shown when the option is disabled")
	}
	if !bytes.Contains([]byte(symlinkRow), []byte("bin")) {
		t.Error("symlink row should still show the name")
	}
}

func TestSymlinkTargetInfoLine(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)
	ui.SetShowSymlinkTarget(true)

	line := ui.symlinkTargetInfoLine(newSymlinkTestFile())
	if !bytes.Contains([]byte(line), []byte("Target:")) {
		t.Errorf("info line should contain 'Target:', got %q", line)
	}
	if !bytes.Contains([]byte(line), []byte("usr/bin")) {
		t.Errorf("info line should contain 'usr/bin', got %q", line)
	}
}

func TestSymlinkTargetInfoLineDisabledByDefault(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)

	if line := ui.symlinkTargetInfoLine(newSymlinkTestFile()); line != "" {
		t.Errorf("info line should be empty when the option is disabled, got %q", line)
	}
}

func TestSymlinkTargetInfoLineForNonSymlink(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)
	ui.SetShowSymlinkTarget(true)

	regular := &analyze.File{Name: "file", Size: 5}
	if line := ui.symlinkTargetInfoLine(regular); line != "" {
		t.Errorf("non-symlink should produce empty info line, got %q", line)
	}
}
