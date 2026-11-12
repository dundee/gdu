package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/session"
)

// stopScanHint returns the progress modal line telling the user how to stop
// the scan and keep the results found so far. Ctrl+C is only advertised when
// it is not configured to quit instead.
func (ui *UI) stopScanHint() string {
	if ui.ctrlCQuits {
		return "Press Esc to stop scanning and keep results"
	}
	return "Press Esc or Ctrl+C to stop scanning and keep results"
}

// progressObserver returns the observer showing the progress of a scan in the
// progress modal, and on the disk progress bar when showDiskProgress is set.
func (ui *UI) progressObserver(deviceSize int64, showDiskProgress bool) session.Observer {
	color := "[white:black:b]"
	if ui.UseColors {
		color = "[red:black:b]"
	}

	start := time.Now()
	lastRoot := 1

	return func(p session.Progress) {
		if p.Phase == session.Finalizing {
			ui.app.QueueUpdateDraw(func() {
				ui.progress.SetTitle(" Finalizing... ")
				ui.progress.SetText("Calculating disk usage...")
			})
			return
		}

		if showDiskProgress {
			percent := int(p.TotalUsage * 100 / deviceSize)
			writeTerminalProgress(percent)
			if ui.progressBar != nil {
				ui.progressBar.SetProgress(percent)
			}
		}

		var title string
		if p.Roots > 1 && p.Root != lastRoot {
			lastRoot = p.Root
			title = scanProgressTitle(p.Root, p.Roots)
		}

		delta := time.Since(start).Round(time.Second)
		ui.app.QueueUpdateDraw(func() {
			if title != "" && !ui.scanCancelled {
				ui.progress.SetTitle(title)
			}
			ui.progress.SetText("Total items: " +
				color +
				common.FormatNumber(p.ItemCount) +
				"[white:black:-], size: " +
				color +
				ui.formatSize(p.TotalUsage, false, false) +
				"[white:black:-], elapsed time: " +
				color +
				delta.String() +
				"[white:black:-]\n\nPress Tab to preview results found so far\n" +
				ui.stopScanHint())
		})
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
