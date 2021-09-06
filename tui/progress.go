package tui

import (
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
)

func (ui *UI) updateProgress() {
	color := "[white:black:b]"
	if ui.UseColors {
		color = "[red:black:b]"
	}

	progressChan := ui.Analyzer.GetProgressChan()
	doneChan := ui.Analyzer.GetDoneChan()

	var progress analyze.CurrentProgress

	for {
		select {
		case progress = <-progressChan:
		case <-doneChan:
			return
		}

		func(itemCount int, totalSize int64, currentItem string) {
			ui.app.QueueUpdateDraw(func() {
				ui.progress.SetText("Total items: " +
					color +
					common.FormatNumber(int64(itemCount)) +
					"[white:black:-] size: " +
					color +
					ui.formatSize(totalSize, false, false) +
					"[white:black:-]\nCurrent item: [white:black:b]" +
					currentItem)
			})
		}(progress.ItemCount, progress.TotalSize, progress.CurrentItemName)

		time.Sleep(100 * time.Millisecond)
	}
}
