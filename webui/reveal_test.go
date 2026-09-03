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
