package webui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Revealing selects the item in its directory where the file manager can,
// and never opens it: opening an .app bundle on macOS launches it.
func TestRevealCommand(t *testing.T) {
	cases := []struct {
		goos     string
		isDir    bool
		wantName string
		wantArgs []string
	}{
		{"darwin", true, "open", []string{"-R", "/tmp/Foo.app"}},
		{"darwin", false, "open", []string{"-R", "/tmp/Foo.app"}},
		{"windows", false, "explorer", []string{"/select,", "/tmp/Foo.app"}},
		{"linux", true, "xdg-open", []string{"/tmp/Foo.app"}},
		{"linux", false, "xdg-open", []string{"/tmp"}},
	}
	for _, c := range cases {
		name, args := revealCommand(c.goos, "/tmp/Foo.app", c.isDir)
		assert.Equal(t, c.wantName, name, c.goos)
		assert.Equal(t, c.wantArgs, args, c.goos)
	}
}

func TestRevealInFileManagerRejectsRelativePath(t *testing.T) {
	assert.EqualError(
		t,
		revealInFileManager("--new-window"),
		`refusing to reveal non-absolute path "--new-window"`,
	)
	assert.EqualError(
		t,
		revealInFileManager("relative/path"),
		`refusing to reveal non-absolute path "relative/path"`,
	)
}
