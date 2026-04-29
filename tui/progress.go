package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/path"
)

func (ui *UI) updateProgress(analyzer common.Analyzer, doneChan common.SignalGroup) {
	color := "[white:black:b]"
	if ui.UseColors {
		color = "[red:black:b]"
	}

	deviceSize := ui.currentDeviceSize
	showBar := ui.showDiskProgressBar

	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
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
		case <-ticker.C:
		}

		progress := analyzer.GetProgress()

		func(itemCount int64, totalUsage int64, currentItem string) {
			delta := time.Since(start).Round(time.Second)

			if deviceSize > 0 && showBar {
				percent := int(totalUsage * 100 / deviceSize)
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
					ui.formatSize(totalUsage, false, false) +
					"[white:black:-], elapsed time: " +
					color +
					delta.String() +
					"[white:black:-]\nCurrent item: [white:black:b]" +
					path.ShortenPath(currentItem, ui.currentItemNameMaxLen))
			})
		}(progress.ItemCount, progress.TotalUsage, progress.CurrentItemName)
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
