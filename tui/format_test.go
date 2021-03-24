package tui

import (
	"testing"

	"github.com/dundee/gdu/v4/internal/testapp"
	"github.com/stretchr/testify/assert"
)

func TestFormatSize(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, false, false)

	assert.Equal(t, "1[white:black:-] B", ui.formatSize(1, false, false))
	assert.Equal(t, "1.0[white:black:-] KiB", ui.formatSize(1<<10, false, false))
	assert.Equal(t, "1.0[white:black:-] MiB", ui.formatSize(1<<20, false, false))
	assert.Equal(t, "1.0[white:black:-] GiB", ui.formatSize(1<<30, false, false))
	assert.Equal(t, "1.0[white:black:-] TiB", ui.formatSize(1<<40, false, false))
	assert.Equal(t, "1.0[white:black:-] PiB", ui.formatSize(1<<50, false, false))
	assert.Equal(t, "1.0[white:black:-] EiB", ui.formatSize(1<<60, false, false))
}
