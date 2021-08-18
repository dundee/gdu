package tui

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/stretchr/testify/assert"
)

func TestFormatSize(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false)

	assert.Equal(t, "1[white:black:-] B", ui.formatSize(1, false, false))
	assert.Equal(t, "1.0[white:black:-] KiB", ui.formatSize(1<<10, false, false))
	assert.Equal(t, "1.0[white:black:-] MiB", ui.formatSize(1<<20, false, false))
	assert.Equal(t, "1.0[white:black:-] GiB", ui.formatSize(1<<30, false, false))
	assert.Equal(t, "1.0[white:black:-] TiB", ui.formatSize(1<<40, false, false))
	assert.Equal(t, "1.0[white:black:-] PiB", ui.formatSize(1<<50, false, false))
	assert.Equal(t, "1.0[white:black:-] EiB", ui.formatSize(1<<60, false, false))
}

func TestFormatCount(t *testing.T) {
	simScreen := testapp.CreateSimScreen(50, 50)
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false)

	assert.Equal(t, "1[-::]", ui.formatCount(1))
	assert.Equal(t, "1.0[-::]k", ui.formatCount(1<<10))
	assert.Equal(t, "1.0[-::]M", ui.formatCount(1<<20))
	assert.Equal(t, "1.1[-::]G", ui.formatCount(1<<30))
}
