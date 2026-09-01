package webui

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRevealCommand(t *testing.T) {
	path := "/tmp/example"
	if runtime.GOOS == "windows" {
		path = `C:\example`
	}

	wantName := "xdg-open"
	switch runtime.GOOS {
	case "darwin":
		wantName = "open"
	case "windows":
		wantName = "explorer"
	}

	name, args := revealCommand(path)
	assert.Equal(t, wantName, name)
	assert.Equal(t, []string{path}, args)
}
