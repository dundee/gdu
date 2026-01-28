package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestHelpNoSpawnShell(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.SetNoSpawnShell()
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	helpText := ui.formatHelpTextFor()

	assert.True(t, strings.Contains(helpText, "Spawn shell in current directory (disabled)"))
	assert.True(t, strings.Contains(helpText, "Open file or directory in external program (disabled)"))
}

func TestCollapsePathFlag(t *testing.T) {
	app := testapp.CreateMockedApp(true)
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)

	// Create a collapsible structure
	deepestDir := &analyze.Dir{
		File: &analyze.File{
			Name:  "deepest",
			Usage: 100,
			Size:  100,
		},
		Files: []fs.Item{},
	}
	middleDir := &analyze.Dir{
		File: &analyze.File{
			Name:  "middle",
			Usage: 100,
			Size:  100,
		},
		Files: []fs.Item{deepestDir},
	}
	topDir := &analyze.Dir{
		File: &analyze.File{
			Name: "top",
		},
		Files: []fs.Item{middleDir},
	}
	deepestDir.SetParent(middleDir)
	middleDir.SetParent(topDir)

	ui.currentDir = topDir
	ui.topDir = topDir
	ui.topDirPath = "top"

	// Default (flag false) -> Should NOT collapse
	ui.showDir()
	cell := ui.table.GetCell(0, 0)
	assert.Contains(t, cell.Text, "middle")
	assert.NotContains(t, cell.Text, "deepest")

	// Enable flag -> Should collapse
	ui.SetCollapsePath(true)
	ui.showDir()
	cell = ui.table.GetCell(0, 0)
	assert.Contains(t, cell.Text, "middle/deepest")
}
