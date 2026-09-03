package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestHelpMoveToTrash(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	helpText := ui.formatHelpTextFor()

	assert.True(t, strings.Contains(helpText, "Move file or directory to trash"))
	assert.False(t, strings.Contains(helpText, "Move file or directory to trash (disabled)"))
}

func TestHelpListsCopyPath(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	helpText := ui.formatHelpTextFor()

	assert.True(t, strings.Contains(helpText, "Copy path of file or directory to clipboard"))
}

func TestHelpNoSpawnShell(t *testing.T) {
	app, simScreen := testapp.CreateTestAppWithSimScreen(50, 50)
	defer simScreen.Fini()

	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)
	ui.SetNoDelete()
	ui.SetNoSpawnShell()
	ui.SetNoViewFile()
	ui.showHelp()

	assert.True(t, ui.pages.HasPage("help"))

	helpText := ui.formatHelpTextFor()

	assert.True(t, strings.Contains(helpText, "Delete file or directory (disabled)"))
	assert.True(t, strings.Contains(helpText, "Empty file or directory (disabled)"))
	assert.True(t, strings.Contains(helpText, "Move file or directory to trash (disabled)"))
	assert.True(t, strings.Contains(helpText, "Spawn shell in current directory (disabled)"))
	assert.True(t, strings.Contains(helpText, "Open file or directory in external program (disabled)"))
	assert.True(t, strings.Contains(helpText, "Show content of file (disabled)"))
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

// TestColumnsWidthMatchesRenderedRow pins ui.columnsWidth() to the width the
// columns actually occupy on screen, for every combination of optional columns.
// The "/.." row is padded with columnsWidth() spaces, so if the two drift apart
// that row stops lining up with the listing below it.
func TestColumnsWidthMatchesRenderedRow(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false)

	root := &analyze.Dir{
		File:      &analyze.File{Name: "root", Usage: 1000, Size: 1000},
		ItemCount: 1,
		BasePath:  ".",
	}
	sub := &analyze.Dir{
		File:      &analyze.File{Name: "subdir", Usage: 700, Size: 700, Parent: root, Flag: ' '},
		ItemCount: 11,
	}
	root.Files = fs.Files{sub}

	maxima := rowMaxima{usage: 1000, size: 1000, count: 10}

	for _, tc := range []struct {
		name                                               string
		oldBar, percentage, count, countBar, mtime, marked bool
	}{
		{name: "defaults"},
		{name: "old bar", oldBar: true},
		{name: "percentage", percentage: true},
		{name: "count", count: true},
		{name: "count and bar", count: true, countBar: true},
		{name: "count bar without count column", countBar: true},
		{name: "count bar with old bar", count: true, countBar: true, oldBar: true},
		{name: "mtime", mtime: true},
		{name: "marked", marked: true},
		{
			name: "everything", oldBar: true, percentage: true,
			count: true, countBar: true, mtime: true, marked: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ui.useOldSizeBar = tc.oldBar
			ui.showBarPercentage = tc.percentage
			ui.showItemCount = tc.count
			ui.showItemCountBar = tc.countBar
			ui.showMtime = tc.mtime
			ui.markedRows = map[int]struct{}{}
			if tc.marked {
				ui.markedRows[0] = struct{}{}
			}

			rendered := ui.formatColumns(sub, maxima, false, false)

			assert.Equal(t, ui.columnsWidth(), tview.TaggedStringWidth(rendered))
		})
	}
}

// TestParentRowIsPaddedToColumnsWidth checks the "/.." row is indented by
// exactly the width of the columns it has none of.
func TestParentRowIsPaddedToColumnsWidth(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false)
	ui.showItemCount = true
	ui.showItemCountBar = true

	root := &analyze.Dir{
		File:      &analyze.File{Name: "root", Usage: 1000, Size: 1000},
		ItemCount: 1,
		BasePath:  ".",
	}
	root.Files = fs.Files{
		&analyze.Dir{
			File:      &analyze.File{Name: "subdir", Usage: 700, Size: 700, Parent: root, Flag: ' '},
			ItemCount: 11,
		},
	}

	ui.currentDir = root
	ui.topDir = root
	// differs from the current dir, so the "/.." row is rendered
	ui.topDirPath = "/somewhere/else"

	ui.showDir()

	parentRow := ui.table.GetCell(0, 0).Text

	assert.Equal(t, strings.Repeat(" ", ui.columnsWidth())+"[::b]/..", parentRow)
}
