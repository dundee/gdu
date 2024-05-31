package tui

import (
	"bufio"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/dundee/gdu/v5/build"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func (ui *UI) showFile() *tview.TextView {
	if ui.currentDir == nil {
		return nil
	}

	row, column := ui.table.GetSelection()
	selectedFile := ui.table.GetCell(row, column).GetReference().(fs.Item)
	if selectedFile.IsDir() {
		return nil
	}

	f, err := os.Open(selectedFile.GetPath())
	if err != nil {
		ui.showErr("Error opening file", err)
		return nil
	}

	totalLines := 0
	scanner := bufio.NewScanner(f)

	file := tview.NewTextView()
	ui.currentDirLabel.SetText("[::b] --- " +
		strings.TrimPrefix(selectedFile.GetPath(), build.RootPathPrefix) +
		" ---").SetDynamicColors(true)

	readNextPart := func(linesCount int) int {
		var err error
		readLines := 0
		for scanner.Scan() && readLines <= linesCount {
			_, err = file.Write(scanner.Bytes())
			if err != nil {
				ui.showErr("Error reading file", err)
				return 0
			}
			_, err = file.Write([]byte("\n"))
			if err != nil {
				ui.showErr("Error reading file", err)
				return 0
			}
			readLines++
		}
		return readLines
	}
	totalLines += readNextPart(defaultLinesCount)

	file.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyESC {
			err = f.Close()
			if err != nil {
				ui.showErr("Error closing file", err)
				return event
			}
			ui.currentDirLabel.SetText("[::b] --- " +
				strings.TrimPrefix(ui.currentDirPath, build.RootPathPrefix) +
				" ---").SetDynamicColors(true)
			ui.pages.RemovePage("file")
			ui.app.SetFocus(ui.table)
			return event
		}

		if event.Rune() == 'j' || event.Rune() == 'G' ||
			event.Key() == tcell.KeyDown || event.Key() == tcell.KeyPgDn {
			_, _, _, height := file.GetInnerRect()
			row, _ := file.GetScrollOffset()
			if height+row > totalLines-linesTreshold {
				totalLines += readNextPart(defaultLinesCount)
			}
		}
		return event
	})

	grid := tview.NewGrid().SetRows(1, 1, 0, 1).SetColumns(0)
	grid.AddItem(ui.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.currentDirLabel, 1, 0, 1, 1, 0, 0, false).
		AddItem(file, 2, 0, 1, 1, 0, 0, true).
		AddItem(ui.footerLabel, 3, 0, 1, 1, 0, 0, false)

	ui.pages.HidePage("background")
	ui.pages.AddPage("file", grid, true, true)

	return file
}
