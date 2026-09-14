package webui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRevealCommand(t *testing.T) {
	cases := []struct {
		goos     string
		wantName string
	}{
		{"darwin", "open"},
		{"windows", "explorer"},
		{"linux", "xdg-open"},
	}
	for _, c := range cases {
		name, args := revealCommand(c.goos, "/tmp/example")
		assert.Equal(t, c.wantName, name)
		assert.Equal(t, []string{"/tmp/example"}, args)
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
