package tui

import (
	"fmt"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

func (ui *UI) formatFileRow(item fs.Item, maxUsage int64, maxSize int64) string {
	var part int

	if ui.ShowApparentSize {
		part = int(float64(item.GetSize()) / float64(maxSize) * 10.0)
	} else {
		part = int(float64(item.GetUsage()) / float64(maxUsage) * 10.0)
	}

	row := string(item.GetFlag())

	if ui.UseColors {
		row += "[#e67100::b]"
	} else {
		row += "[::b]"
	}

	if ui.ShowApparentSize {
		row += fmt.Sprintf("%15s", ui.formatSize(item.GetSize(), false, true))
	} else {
		row += fmt.Sprintf("%15s", ui.formatSize(item.GetUsage(), false, true))
	}

	row += getUsageGraph(part)

	if ui.showItemCount {
		if ui.UseColors {
			row += "[#e67100::b]"
		} else {
			row += "[::b]"
		}
		row += fmt.Sprintf("%11s ", ui.formatCount(item.GetItemCount()))
	}

	if ui.showMtime {
		if ui.UseColors {
			row += "[#e67100::b]"
		} else {
			row += "[::b]"
		}
		row += fmt.Sprintf(
			"%s [-::]",
			item.GetMtime().Format("2006-01-02 15:04:05"),
		)
	}

	if item.IsDir() {
		if ui.UseColors {
			row += "[#3498db::b]/"
		} else {
			row += "[::b]/"
		}
	}
	row += tview.Escape(item.GetName())
	return row
}

func (ui *UI) formatSize(size int64, reverseColor bool, transparentBg bool) string {
	var color string
	if reverseColor {
		if ui.UseColors {
			color = "[black:#2479d0:-]"
		} else {
			color = "[black:white:-]"
		}
	} else {
		if transparentBg {
			color = "[-::]"
		} else {
			color = "[white:black:-]"
		}
	}

	if ui.UseSIPrefix {
		return formatWithDecPrefix(size, color)
	}
	return formatWithBinPrefix(float64(size), color)
}

func (ui *UI) formatCount(count int) string {
	row := ""
	color := "[-::]"
	count64 := int64(count)

	switch {
	case count64 >= common.G:
		row += fmt.Sprintf("%.1f%sG", float64(count)/float64(common.G), color)
	case count64 >= common.M:
		row += fmt.Sprintf("%.1f%sM", float64(count)/float64(common.M), color)
	case count64 >= common.K:
		row += fmt.Sprintf("%.1f%sk", float64(count)/float64(common.K), color)
	default:
		row += fmt.Sprintf("%d%s", count, color)
	}
	return row
}

func formatWithBinPrefix(fsize float64, color string) string {
	switch {
	case fsize >= common.Ei:
		return fmt.Sprintf("%.1f%s EiB", fsize/common.Ei, color)
	case fsize >= common.Pi:
		return fmt.Sprintf("%.1f%s PiB", fsize/common.Pi, color)
	case fsize >= common.Ti:
		return fmt.Sprintf("%.1f%s TiB", fsize/common.Ti, color)
	case fsize >= common.Gi:
		return fmt.Sprintf("%.1f%s GiB", fsize/common.Gi, color)
	case fsize >= common.Mi:
		return fmt.Sprintf("%.1f%s MiB", fsize/common.Mi, color)
	case fsize >= common.Ki:
		return fmt.Sprintf("%.1f%s KiB", fsize/common.Ki, color)
	default:
		return fmt.Sprintf("%d%s B", int64(fsize), color)
	}
}

func formatWithDecPrefix(size int64, color string) string {
	fsize := float64(size)
	switch {
	case size >= common.E:
		return fmt.Sprintf("%.1f%s EB", fsize/float64(common.E), color)
	case size >= common.P:
		return fmt.Sprintf("%.1f%s PB", fsize/float64(common.P), color)
	case size >= common.T:
		return fmt.Sprintf("%.1f%s TB", fsize/float64(common.T), color)
	case size >= common.G:
		return fmt.Sprintf("%.1f%s GB", fsize/float64(common.G), color)
	case size >= common.M:
		return fmt.Sprintf("%.1f%s MB", fsize/float64(common.M), color)
	case size >= common.K:
		return fmt.Sprintf("%.1f%s kB", fsize/float64(common.K), color)
	default:
		return fmt.Sprintf("%d%s B", size, color)
	}
}
