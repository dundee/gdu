package tui

import (
	"fmt"
	"math"

	"github.com/dundee/gdu/analyze"
)

func (ui *UI) formatFileRow(item *analyze.File) string {
	var part int

	if ui.showApparentSize {
		part = int(float64(item.Size) / float64(item.Parent.Size) * 10.0)
	} else {
		part = int(float64(item.Usage) / float64(item.Parent.Usage) * 10.0)
	}

	row := string(item.Flag)

	if ui.useColors {
		row += "[#e67100:-:b]"
	} else {
		row += "[white:-:b]"
	}

	if ui.showApparentSize {
		row += fmt.Sprintf("%21s", ui.formatSize(item.Size, false, true))
	} else {
		row += fmt.Sprintf("%21s", ui.formatSize(item.Usage, false, true))
	}

	row += getUsageGraph(part)

	if item.IsDir {
		if ui.useColors {
			row += "[#3498db::b]/"
		} else {
			row += "[::b]/"
		}
	}
	row += item.Name
	return row
}

func (ui *UI) formatSize(size int64, reverseColor bool, transparentBg bool) string {
	var color string
	if reverseColor {
		if ui.useColors {
			color = "[black:#2479d0:-]"
		} else {
			color = "[black:white:-]"
		}
	} else {
		if transparentBg {
			color = "[white:-:-]"
		} else {
			color = "[white:black:-]"
		}
	}

	switch {
	case size > 1e12:
		return fmt.Sprintf("%.1f%s TiB", float64(size)/math.Pow(2, 40), color)
	case size > 1e9:
		return fmt.Sprintf("%.1f%s GiB", float64(size)/math.Pow(2, 30), color)
	case size > 1e6:
		return fmt.Sprintf("%.1f%s MiB", float64(size)/math.Pow(2, 20), color)
	case size > 1e3:
		return fmt.Sprintf("%.1f%s KiB", float64(size)/math.Pow(2, 10), color)
	default:
		return fmt.Sprintf("%d%s B", size, color)
	}
}
