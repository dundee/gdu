package tui

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func TestSetTrashCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on the unix 'true' command")
	}

	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)

	dir := &analyze.Dir{
		File:      &analyze.File{Name: "test_dir"},
		ItemCount: 2,
		BasePath:  ".",
	}
	file := &analyze.File{Name: "file2", Parent: dir}
	dir.Files = fs.Files{file}

	ui.SetTrashCommand([]string{"true"})
	err := ui.trasher(dir, file)
	require.NoError(t, err)
	assert.Equal(t, 0, len(dir.Files))
}
