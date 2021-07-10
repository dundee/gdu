package tui

import (
	"fmt"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
)

func (ui *UI) formatFileRow(item analyze.Item) string {
	var part int

	if ui.ShowApparentSize {
		part = int(float64(item.GetSize()) / float64(item.GetParent().GetSize()) * 10.0)
	} else {
		part = int(float64(item.GetUsage()) / float64(item.GetParent().GetUsage()) * 10.0)
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

	if item.IsDir() {
		if ui.UseColors {
			row += "[#3498db::b]/"
		} else {
			row += "[::b]/"
		}
	}
	row += item.GetName()
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

	fsize := float64(size)

	switch {
	case fsize >= common.EB:
		return fmt.Sprintf("%.1f%s EiB", fsize/common.EB, color)
	case fsize >= common.PB:
		return fmt.Sprintf("%.1f%s PiB", fsize/common.PB, color)
	case fsize >= common.TB:
		return fmt.Sprintf("%.1f%s TiB", fsize/common.TB, color)
	case fsize >= common.GB:
		return fmt.Sprintf("%.1f%s GiB", fsize/common.GB, color)
	case fsize >= common.MB:
		return fmt.Sprintf("%.1f%s MiB", fsize/common.MB, color)
	case fsize >= common.KB:
		return fmt.Sprintf("%.1f%s KiB", fsize/common.KB, color)
	default:
		return fmt.Sprintf("%d%s B", size, color)
	}
}

func (ui *UI) formatCount(count int) string {
	row := ""
	color := "[-::]"

	switch {
	case count >= common.G:
		row += fmt.Sprintf("%.1f%sG", float64(count)/float64(common.G), color)
	case count >= common.M:
		row += fmt.Sprintf("%.1f%sM", float64(count)/float64(common.M), color)
	case count >= common.K:
		row += fmt.Sprintf("%.1f%sk", float64(count)/float64(common.K), color)
	default:
		row += fmt.Sprintf("%d%s", count, color)
	}
	return row
}
