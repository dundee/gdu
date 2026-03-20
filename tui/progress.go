package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/path"
)

func (ui *UI) updateProgress() {
	color := "[white:black:b]"
	if ui.UseColors {
		color = "[red:black:b]"
	}

	progressChan := ui.Analyzer.GetProgressChan()
	doneChan := ui.Analyzer.GetDone()
	deviceSize := ui.currentDeviceSize
	showBar := ui.showDiskProgressBar

	var progress common.CurrentProgress
	start := time.Now()

	for {
		select {
		case progress = <-progressChan:
		case <-doneChan:
			if deviceSize > 0 && showBar {
				clearTerminalProgress()
				ui.currentDeviceSize = 0
			}
			ui.app.QueueUpdateDraw(func() {
				ui.progress.SetTitle(" Finalizing... ")
				ui.progress.SetText("Calculating disk usage...")
			})
			return
		}

		func(itemCount int64, totalSize int64, currentItem string) {
			delta := time.Since(start).Round(time.Second)

			if deviceSize > 0 && showBar {
				percent := int(totalSize * 100 / deviceSize)
				writeTerminalProgress(percent)
				if ui.progressBar != nil {
					ui.progressBar.SetProgress(percent)
				}
			}

			ui.app.QueueUpdateDraw(func() {
				ui.progress.SetText("Total items: " +
					color +
					common.FormatNumber(int64(itemCount)) +
					"[white:black:-], size: " +
					color +
					ui.formatSize(totalSize, false, false) +
					"[white:black:-], elapsed time: " +
					color +
					delta.String() +
					"[white:black:-]\nCurrent item: [white:black:b]" +
					path.ShortenPath(currentItem, ui.currentItemNameMaxLen))
			})
		}(progress.ItemCount, progress.TotalSize, progress.CurrentItemName)

		time.Sleep(100 * time.Millisecond)
	}
}

// writeTerminalProgress emits an OSC 9;4 sequence to update the terminal
// tab/taskbar progress indicator. percent must be in the range [0, 100].
// This sequence is supported by Windows Terminal, ConEmu, and compatible
// terminals.  Writing to stderr ensures it reaches the terminal even when
// the TUI has taken over stdout/stdin via tcell.
func writeTerminalProgress(percent int) {
	fmt.Fprintf(os.Stderr, "\x1b]9;4;1;%d\x1b\\", percent)
}

// clearTerminalProgress removes the terminal tab/taskbar progress indicator.
func clearTerminalProgress() {
	fmt.Fprintf(os.Stderr, "\x1b]9;4;0;0\x1b\\")
}
