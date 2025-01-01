package tui

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/pkg/errors"
	"github.com/rivo/tview"
	"github.com/ulikunitz/xz"

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

	path := selectedFile.GetPath()
	f, err := os.Open(path)
	if err != nil {
		ui.showErr("Error opening file", err)
		return nil
	}
	scanner, err := getScanner(f)
	if err != nil {
		ui.showErr("Error reading file", err)
		return nil
	}

	totalLines := 0

	file := tview.NewTextView()
	ui.currentDirLabel.SetText("[::b] --- " +
		strings.TrimPrefix(path, build.RootPathPrefix) +
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
			if height+row > totalLines-linesThreshold {
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

func getScanner(f io.ReadSeeker) (scanner *bufio.Scanner, err error) {
	// We only have to pass the file header = first 261 bytes
	head := make([]byte, 261)
	if _, err = f.Read(head); err != nil {
		return nil, errors.Wrap(err, "error reading file header")
	}

	if pos, err := f.Seek(0, 0); pos != 0 || err != nil {
		return nil, errors.Wrap(err, "error seeking file")
	}
	scanner = bufio.NewScanner(f)

	typ, err := filetype.Match(head)
	if err != nil {
		return nil, errors.Wrap(err, "error matching file type")
	}

	switch typ.MIME.Value {
	case matchers.TypeGz.MIME.Value:
		r, err := gzip.NewReader(f)
		if err != nil {
			return nil, errors.Wrap(err, "error creating gzip reader")
		}
		scanner = bufio.NewScanner(r)
	case matchers.TypeBz2.MIME.Value:
		r := bzip2.NewReader(f)
		scanner = bufio.NewScanner(r)
	case matchers.TypeXz.MIME.Value:
		r, err := xz.NewReader(f)
		if err != nil {
			return nil, errors.Wrap(err, "error creating xz reader")
		}
		scanner = bufio.NewScanner(r)
	}

	return scanner, nil
}
