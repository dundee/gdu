package tui

import (
	"fmt"
	"math"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

const (
	blackOnWhite = "[black:white:-]"
	whiteOnBlack = "[white:black:-]"

	defaultColor     = "[-::]"
	defaultColorBold = "[::b]"
)

func (ui *UI) formatFileRow(item fs.Item, maxUsage, maxSize int64, marked, ignored bool) string {
	part := 0
	if !ignored {
		if ui.ShowApparentSize {
			if size := item.GetSize(); size > 0 {
				part = int(float64(size) / float64(maxSize) * 100.0)
			}
		} else {
			if usage := item.GetUsage(); usage > 0 {
				part = int(float64(usage) / float64(maxUsage) * 100.0)
			}
		}
	}

	row := string(item.GetFlag())

	numberColor := fmt.Sprintf(
		"[%s::b]",
		ui.resultRow.NumberColor,
	)

	if ui.UseColors && !marked && !ignored {
		row += numberColor
	} else {
		row += defaultColorBold
	}

	if ui.ShowApparentSize {
		row += fmt.Sprintf("%15s", ui.formatSize(item.GetSize(), false, true))
	} else {
		row += fmt.Sprintf("%15s", ui.formatSize(item.GetUsage(), false, true))
	}

	if ui.useOldSizeBar {
		row += " " + getUsageGraphOld(part) + " "
	} else {
		row += getUsageGraph(part)
	}

	if ui.showItemCount {
		if ui.UseColors && !marked && !ignored {
			row += numberColor
		} else {
			row += defaultColorBold
		}
		row += fmt.Sprintf("%11s ", ui.formatCount(item.GetItemCount()))
	}

	if ui.showMtime {
		if ui.UseColors && !marked && !ignored {
			row += numberColor
		} else {
			row += defaultColorBold
		}
		row += fmt.Sprintf(
			"%s "+defaultColor,
			item.GetMtime().Format("2006-01-02 15:04:05"),
		)
	}

	if len(ui.markedRows) > 0 {
		if marked {
			row += string('✓')
		} else {
			row += " "
		}
		row += " "
	}

	if item.IsDir() {
		if ui.UseColors && !marked && !ignored {
			row += fmt.Sprintf("[%s::b]/", ui.resultRow.DirectoryColor)
		} else {
			row += defaultColorBold + "/"
		}
	}
	row += tview.Escape(item.GetName())
	return row
}

func (ui *UI) formatSize(size int64, reverseColor, transparentBg bool) string {
	var color string
	if reverseColor {
		if ui.UseColors {
			color = fmt.Sprintf(
				"[%s:%s:-]",
				ui.footerTextColor,
				ui.footerBackgroundColor,
			)
		} else {
			color = blackOnWhite
		}
	} else {
		if transparentBg {
			color = defaultColor
		} else {
			color = whiteOnBlack
		}
	}

	if ui.UseSIPrefix {
		return formatWithDecPrefix(size, color)
	}
	return formatWithBinPrefix(float64(size), color)
}

func (ui *UI) formatCount(count int) string {
	row := ""
	color := defaultColor
	count64 := float64(count)

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
	asize := math.Abs(fsize)

	switch {
	case asize >= common.Ei:
		return fmt.Sprintf("%.1f%s EiB", fsize/common.Ei, color)
	case asize >= common.Pi:
		return fmt.Sprintf("%.1f%s PiB", fsize/common.Pi, color)
	case asize >= common.Ti:
		return fmt.Sprintf("%.1f%s TiB", fsize/common.Ti, color)
	case asize >= common.Gi:
		return fmt.Sprintf("%.1f%s GiB", fsize/common.Gi, color)
	case asize >= common.Mi:
		return fmt.Sprintf("%.1f%s MiB", fsize/common.Mi, color)
	case asize >= common.Ki:
		return fmt.Sprintf("%.1f%s KiB", fsize/common.Ki, color)
	default:
		return fmt.Sprintf("%d%s B", int64(fsize), color)
	}
}

func formatWithDecPrefix(size int64, color string) string {
	fsize := float64(size)
	asize := math.Abs(fsize)
	switch {
	case asize >= common.E:
		return fmt.Sprintf("%.1f%s EB", fsize/common.E, color)
	case asize >= common.P:
		return fmt.Sprintf("%.1f%s PB", fsize/common.P, color)
	case asize >= common.T:
		return fmt.Sprintf("%.1f%s TB", fsize/common.T, color)
	case asize >= common.G:
		return fmt.Sprintf("%.1f%s GB", fsize/common.G, color)
	case asize >= common.M:
		return fmt.Sprintf("%.1f%s MB", fsize/common.M, color)
	case asize >= common.K:
		return fmt.Sprintf("%.1f%s kB", fsize/common.K, color)
	default:
		return fmt.Sprintf("%d%s B", size, color)
	}
}
