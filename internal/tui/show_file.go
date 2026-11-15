package tui

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/rivo/tview"
	"github.com/ulikunitz/xz"

	"github.com/dundee/gdu/v6/build"
	"github.com/dundee/gdu/v6/internal/xzcheck"
	"github.com/dundee/gdu/v6/pkg/fs"
)

func (ui *UI) showFile() *tview.TextView {
	if ui.currentDir == nil {
		return nil
	}

	row, column := ui.table.GetSelection()
	cell := ui.table.GetCell(row, column)
	if cell == nil || cell.GetReference() == nil {
		return nil
	}

	selectedFile, ok := cell.GetReference().(fs.Item)
	if !ok || selectedFile == nil || selectedFile.IsDir() {
		return nil
	}

	path := selectedFile.GetPath()
	f, scanner, ok := ui.openViewedFile(path)
	if !ok {
		return nil
	}

	totalLines := 0

	file := tview.NewTextView()
	ui.currentDirLabel.SetText("[::b] --- " +
		tview.Escape(strings.TrimPrefix(path, build.RootPathPrefix)) +
		" ---").SetDynamicColors(true)

	reported := false
	readNextPart := func(linesCount int) int {
		var err error
		readLines := 0
		defer func() {
			if err := scanner.Err(); err != nil && !reported {
				reported = true
				ui.showErr("Error reading file", err)
			}
		}()
		for readLines <= linesCount && scanner.Scan() {
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
		if typedRune(event) == 'q' || event.Key() == tcell.KeyESC {
			if err := f.Close(); err != nil {
				slog.Warn("closing file", "path", path, "err", err)
			}
			ui.currentDirLabel.SetText("[::b] --- " +
				strings.TrimPrefix(ui.currentDirPath, build.RootPathPrefix) +
				" ---").SetDynamicColors(true)
			ui.pages.RemovePage("file")
			ui.app.SetFocus(ui.table)
			return event
		}

		if typedRune(event) == 'j' || typedRune(event) == 'G' ||
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

// openViewedFile opens the file at path for viewing, and tells the user
// why not when it cannot be.
func (ui *UI) openViewedFile(path string) (*os.File, *bufio.Scanner, bool) {
	// only a regular file: opening a FIFO waits for a writer, on the UI
	// goroutine, and a device such as /dev/zero never ends
	info, err := os.Stat(path)
	if err != nil {
		ui.showErr("Error opening file", err)
		return nil, nil, false
	}
	if !info.Mode().IsRegular() {
		ui.showMessage(" Only regular files can be viewed")
		return nil, nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		ui.showErr("Error opening file", err)
		return nil, nil, false
	}
	scanner, err := getScanner(f)
	if err != nil {
		f.Close()
		ui.showErr("Error reading file", err)
		return nil, nil, false
	}
	return f, scanner, true
}

func getScanner(f io.ReadSeeker) (scanner *bufio.Scanner, err error) {
	// We only have to pass the file header = first 261 bytes
	head := make([]byte, 261)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("error reading file header: %w", err)
	}
	head = head[:n]

	if pos, err := f.Seek(0, io.SeekStart); pos != 0 || err != nil {
		return nil, fmt.Errorf("error seeking file: %w", err)
	}
	scanner = newLineScanner(f)
	if n == 0 {
		// an empty file
		return scanner, nil
	}

	typ, err := filetype.Match(head)
	if err != nil {
		return nil, fmt.Errorf("error matching file type: %w", err)
	}

	switch typ.MIME.Value {
	case matchers.TypeGz.MIME.Value:
		r, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("error creating gzip reader: %w", err)
		}
		scanner = newLineScanner(r)
	case matchers.TypeBz2.MIME.Value:
		r := bzip2.NewReader(f)
		scanner = newLineScanner(r)
	case matchers.TypeXz.MIME.Value:
		if err := checkXZ(f); err != nil {
			return nil, err
		}
		r, err := xz.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("error creating xz reader: %w", err)
		}
		scanner = newLineScanner(r)
	}

	return scanner, nil
}

// checkXZ fails for an xz file asking the decoder for more memory than
// xzcheck.MaxDict.
func checkXZ(f io.ReadSeeker) error {
	r, ok := f.(io.ReaderAt)
	if !ok {
		return nil
	}
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return xzcheck.CheckDicts(r, size, xzcheck.MaxDict)
}

// maxViewedLine is the length a line of a viewed file is cut at.
const maxViewedLine = 4096

// newLineScanner returns a scanner of the lines of r, which cuts a line
// longer than maxViewedLine, as minified or binary files have, where a
// plain bufio.Scanner stops at the first line over 64 KiB.
func newLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := bufio.ScanLines(data, atEOF)
		if advance == 0 && token == nil && err == nil && len(data) >= maxViewedLine {
			return maxViewedLine, data[:maxViewedLine], nil
		}
		return advance, token, err
	})
	return scanner
}
