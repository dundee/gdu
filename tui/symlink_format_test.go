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
