package tui

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// clipboardWriteAll is a package-level indirection over the clipboard writer so
// tests can swap it out without touching a real system clipboard.
var clipboardWriteAll = clipboard.WriteAll

func (ui *UI) copySelectedPath() {
	if ui.currentDir == nil {
		return
	}
	row, column := ui.table.GetSelection()
	selectedFile, ok := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if !ok {
		return
	}

	path := strings.TrimPrefix(selectedFile.GetPath(), build.RootPathPrefix)
	if err := clipboardWriteAll(path); err != nil {
		ui.showErr("Error copying path to clipboard", err)
		return
	}

	// brief confirmation in the header, restored after a moment like other actions
	previousHeaderText := ui.header.GetText(false)
	ui.header.SetText(" Path copied to clipboard")
	go func() {
		time.Sleep(2 * time.Second)
		ui.app.QueueUpdateDraw(func() {
			ui.header.Clear()
			ui.header.SetText(previousHeaderText)
		})
	}()
}
