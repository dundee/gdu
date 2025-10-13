package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/stretchr/testify/assert"
)

func TestHelpNoSpawnShell(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false, false)
	ui.SetNoSpawnShell()
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	helpText := ui.formatHelpTextFor()

	assert.True(t, strings.Contains(helpText, "Spawn shell in current directory (disabled)"))
	assert.True(t, strings.Contains(helpText, "Open file or directory in external program (disabled)"))
}
