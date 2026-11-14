package tui

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Unsetenv("BLOCK_SIZE")
	os.Unsetenv("BLOCKSIZE")
	os.Exit(m.Run())
}

func TestFormatSize(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)

	assert.Equal(t, "1[white:black:-] B", ui.formatSize(1, false, false))
	assert.Equal(t, "1.0[white:black:-] KiB", ui.formatSize(1<<10, false, false))
	assert.Equal(t, "1.0[white:black:-] MiB", ui.formatSize(1<<20, false, false))
	assert.Equal(t, "1.0[white:black:-] GiB", ui.formatSize(1<<30, false, false))
	assert.Equal(t, "1.0[white:black:-] TiB", ui.formatSize(1<<40, false, false))
	assert.Equal(t, "1.0[white:black:-] PiB", ui.formatSize(1<<50, false, false))
	assert.Equal(t, "1.0[white:black:-] EiB", ui.formatSize(1<<60, false, false))
	assert.Equal(t, "-1.0[white:black:-] KiB", ui.formatSize(-1<<10, false, false))
}

func TestFormatSizeWithBlockSizeEnvironment(t *testing.T) {
	t.Setenv("BLOCK_SIZE", "1K")
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	ui := CreateUI(testapp.CreateMockedApp(true), simScreen, &bytes.Buffer{}, false, false, false, false)
	assert.Equal(t, "2[white:black:-]", ui.formatSize(1025, false, false))
}

func TestFormatSizeDec(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, true)

	assert.Equal(t, "1[white:black:-] B", ui.formatSize(1, false, false))
	assert.Equal(t, "1.0[white:black:-] kB", ui.formatSize(1<<10, false, false))
	assert.Equal(t, "1.0[white:black:-] MB", ui.formatSize(1<<20, false, false))
	assert.Equal(t, "1.1[white:black:-] GB", ui.formatSize(1<<30, false, false))
	assert.Equal(t, "1.1[white:black:-] TB", ui.formatSize(1<<40, false, false))
	assert.Equal(t, "1.1[white:black:-] PB", ui.formatSize(1<<50, false, false))
	assert.Equal(t, "1.2[white:black:-] EB", ui.formatSize(1<<60, false, false))
	assert.Equal(t, "-1.0[white:black:-] kB", ui.formatSize(-1<<10, false, false))
}

func TestFormatCount(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)

	assert.Equal(t, "1[-::]", ui.formatCount(1))
	assert.Equal(t, "1.0[-::]k", ui.formatCount(1<<10))
	assert.Equal(t, "1.0[-::]M", ui.formatCount(1<<20))
	assert.Equal(t, "1.1[-::]G", ui.formatCount(1<<30))
}

func TestEscapeName(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)

	dir := &analyze.Dir{
		File: &analyze.File{
			Usage: 10,
		},
	}

	file := &analyze.File{
		Name:   "Aaa [red] bbb",
		Parent: dir,
		Usage:  10,
	}

	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: file.GetUsage(), size: file.GetSize()}, false, false, false), "Aaa [red[] bbb")
}

func TestMarked(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	ui.markedItems[&analyze.File{Name: "marked"}] = struct{}{}
	ui.useOldSizeBar = true

	dir := &analyze.Dir{
		File: &analyze.File{
			Usage: 10,
		},
	}

	file := &analyze.File{
		Name:   "Aaa",
		Parent: dir,
		Usage:  10,
	}

	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: file.GetUsage(), size: file.GetSize()}, true, false, false), "✓ Aaa")
	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: file.GetUsage(), size: file.GetSize()}, false, false, false), "[##########]   Aaa")
}

func TestIgnored(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	ui.ignoredRows[0] = struct{}{}
	ui.useOldSizeBar = true

	dir := &analyze.Dir{
		File: &analyze.File{
			Usage: 10,
		},
	}

	file := &analyze.File{
		Name:   "Aaa",
		Parent: dir,
		Usage:  10,
	}

	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: file.GetUsage(), size: file.GetSize()}, false, true, false), "[          ] Aaa")
	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: file.GetUsage(), size: file.GetSize()}, false, false, false), "[##########] Aaa")
}

func TestSizeBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)

	dir := &analyze.Dir{
		File: &analyze.File{
			Usage: 10,
		},
	}

	file := &analyze.File{
		Name:   "Aaa",
		Parent: dir,
		Usage:  10,
	}

	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: file.GetUsage(), size: file.GetSize()}, false, false, false), "██████████▏Aaa")
}

func TestOldSizeBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	ui.markedItems[&analyze.File{Name: "marked"}] = struct{}{}
	ui.useOldSizeBar = true

	dir := &analyze.Dir{
		File: &analyze.File{
			Usage: 20,
		},
	}

	file := &analyze.File{
		Name:   "Aaa",
		Parent: dir,
		Usage:  10,
	}

	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: dir.GetUsage(), size: dir.GetSize()}, false, false, false), "[#####     ]   Aaa")
}

func TestSizeBarWithPercentage(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	ui.SetShowBarPercentage()

	dir := &analyze.Dir{
		File: &analyze.File{
			Usage: 10,
		},
	}

	file := &analyze.File{
		Name:   "Aaa",
		Parent: dir,
		Usage:  10,
	}

	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: file.GetUsage(), size: file.GetSize()}, false, false, false), "100.0% ██████████▏Aaa")
}

func TestSizeBarWithPercentagePartial(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	ui.SetShowBarPercentage()
	ui.useOldSizeBar = true

	dir := &analyze.Dir{
		File: &analyze.File{
			Usage: 20,
		},
	}

	file := &analyze.File{
		Name:   "Aaa",
		Parent: dir,
		Usage:  10,
	}

	assert.Contains(t, ui.formatFileRow(file, rowMaxima{usage: dir.GetUsage(), size: dir.GetSize()}, false, false, false), "50.0% [#####     ]")
}

func TestItemCountBar(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	ui.useOldSizeBar = true
	ui.showItemCount = true
	ui.showItemCountBar = true

	parent := &analyze.Dir{File: &analyze.File{Name: "parent"}}

	// 3 of the 4 items in the listing, so the bar is 75% full.
	dir := &analyze.Dir{
		File:      &analyze.File{Name: "Aaa", Parent: parent},
		ItemCount: 4,
	}

	row := ui.formatFileRow(dir, rowMaxima{count: 4}, false, false, false)

	assert.Contains(t, row, "3")
	assert.Contains(t, row, "[#######   ]")
}

func TestFormatQueuedRowUsesCellColorInsteadOfTags(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)
	ui.SetResultRowDirectoryColor("#3498db")
	ui.SetResultRowNumberColor("#e67100")

	parent := &analyze.Dir{File: &analyze.File{Name: "parent"}}
	dir := &analyze.Dir{File: &analyze.File{Name: "Aaa", Parent: parent}}

	normal := ui.formatFileRow(dir, rowMaxima{}, false, false, false)
	assert.Contains(t, normal, "#3498db")
	assert.Contains(t, normal, "#e67100")

	// A queued row is coloured by the cell style, so its text must not carry
	// colour tags that would override it.
	queued := ui.formatFileRow(dir, rowMaxima{}, false, false, true)
	assert.NotContains(t, queued, "#3498db")
	assert.NotContains(t, queued, "#e67100")
}

func TestItemCountBarHiddenWithoutItemCount(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	ui.useOldSizeBar = true
	ui.showItemCountBar = true

	parent := &analyze.Dir{File: &analyze.File{Name: "parent"}}
	dir := &analyze.Dir{
		File:      &analyze.File{Name: "Aaa", Parent: parent},
		ItemCount: 4,
	}

	// The count column is hidden, so only the size bar is drawn.
	row := ui.formatFileRow(dir, rowMaxima{count: 4}, false, false, false)

	assert.Equal(t, 1, strings.Count(row, "["+"          ]"))
}

func TestItemCountBarIgnoredRowIsEmpty(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	ui.useOldSizeBar = true
	ui.showItemCount = true
	ui.showItemCountBar = true

	parent := &analyze.Dir{File: &analyze.File{Name: "parent"}}
	dir := &analyze.Dir{
		File:      &analyze.File{Name: "Aaa", Parent: parent},
		ItemCount: 4,
	}

	row := ui.formatFileRow(dir, rowMaxima{count: 4}, false, true, false)

	// Both bars are empty for an ignored row.
	assert.Equal(t, 2, strings.Count(row, "[          ]"))
}

func TestColumnsWidth(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)

	// flag + size + unicode bar
	assert.Equal(t, 23, ui.columnsWidth())

	ui.useOldSizeBar = true
	assert.Equal(t, 25, ui.columnsWidth())

	ui.showItemCount = true
	assert.Equal(t, 32, ui.columnsWidth())

	ui.showItemCountBar = true
	assert.Equal(t, 46, ui.columnsWidth())

	ui.showMtime = true
	assert.Equal(t, 66, ui.columnsWidth())

	ui.markedItems[&analyze.File{Name: "marked"}] = struct{}{}
	assert.Equal(t, 68, ui.columnsWidth())
}
