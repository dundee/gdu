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

// rowMaxima holds the denominators the bars of one directory listing are
// scaled against. Which field is used depends on the displayed size metric and
// on which bar is being drawn. Each field is either the sum over the sibling
// items or, with ShowRelativeSize, the largest sibling.
type rowMaxima struct {
	usage int64
	size  int64
	count int64
}

// getUsagePart returns the percentage (0-100) that the given item's size or
// usage represents relative to the provided maximum.
func (ui *UI) getUsagePart(item fs.Item, maxima rowMaxima, ignored bool) float64 {
	if ignored {
		return 0
	}
	if ui.ShowApparentSize {
		if size := item.GetSize(); size > 0 {
			return float64(size) / float64(maxima.size) * 100.0
		}
	} else {
		if usage := item.GetUsage(); usage > 0 {
			return float64(usage) / float64(maxima.usage) * 100.0
		}
	}
	return 0
}

// getCountPart returns the percentage (0-100) that the given item's displayed
// item count represents relative to the provided maximum.
func (ui *UI) getCountPart(item fs.Item, maxima rowMaxima, ignored bool) float64 {
	if ignored || maxima.count == 0 {
		return 0
	}
	if count := fs.DisplayedItemCount(item); count > 0 {
		return float64(count) / float64(maxima.count) * 100.0
	}
	return 0
}

// formatUsagePercentage formats the numeric usage percentage shown next to the size bar.
func formatUsagePercentage(part float64) string {
	return fmt.Sprintf(" %5.1f%%", part)
}

// formatBar renders a single bar filled to the given percentage, in whichever
// bar style is configured.
func (ui *UI) formatBar(part int) string {
	if ui.useOldSizeBar {
		return " " + getUsageGraphOld(part) + " "
	}
	return getUsageGraph(part)
}

// Visible widths of the columns rendered by formatColumns. The numeric columns
// are padded to a wider format verb than these because formatSize and
// formatCount embed a color tag, which is counted by the verb but occupies no
// screen columns.
const (
	flagColumnWidth       = 1
	sizeColumnWidth       = 10
	percentageColumnWidth = 7
	itemCountColumnWidth  = 7
	mtimeColumnWidth      = 20
	markedColumnWidth     = 2
	barColumnWidth        = 12
	oldBarColumnWidth     = 14
)

// columnsWidth returns the number of screen columns formatColumns occupies, so
// that rows which have no columns of their own (the "/.." entry) can be padded
// to line up their name with the rest of the listing.
func (ui *UI) columnsWidth() int {
	barWidth := barColumnWidth
	if ui.useOldSizeBar {
		barWidth = oldBarColumnWidth
	}

	width := flagColumnWidth + sizeColumnWidth + barWidth
	if ui.showBarPercentage {
		width += percentageColumnWidth
	}
	if ui.showItemCount {
		width += itemCountColumnWidth
		if ui.showItemCountBar {
			width += barWidth
		}
	}
	if ui.showMtime {
		width += mtimeColumnWidth
	}
	if len(ui.markedRows) > 0 {
		width += markedColumnWidth
	}
	return width
}

// formatColumns renders every column of a row except the trailing name: the
// type flag, the size, the size bar, and the optional percentage, item count,
// item count bar, mtime and marked columns.
//
// statsItem is the item the numbers are taken from, which is not always the
// item whose name ends up on the row: a collapsed path displays the name of a
// whole chain of directories but the stats of its deepest one.
func (ui *UI) formatColumns(statsItem fs.Item, maxima rowMaxima, marked, ignored bool) string {
	// numberPrefix is the color tag introducing a numeric column.
	numberPrefix := func() string {
		if ui.UseColors && !marked && !ignored {
			return fmt.Sprintf("[%s::b]", ui.resultRow.NumberColor)
		}
		return defaultColorBold
	}

	row := string(statsItem.GetFlag()) + numberPrefix()

	if ui.ShowApparentSize {
		row += fmt.Sprintf("%15s", ui.formatSize(statsItem.GetSize(), false, true))
	} else {
		row += fmt.Sprintf("%15s", ui.formatSize(statsItem.GetUsage(), false, true))
	}

	usagePart := ui.getUsagePart(statsItem, maxima, ignored)
	if ui.showBarPercentage {
		row += formatUsagePercentage(usagePart)
	}
	row += ui.formatBar(int(usagePart))

	if ui.showItemCount {
		row += numberPrefix()
		row += fmt.Sprintf("%11s ", ui.formatCount(fs.DisplayedItemCount(statsItem)))

		if ui.showItemCountBar {
			row += ui.formatBar(int(ui.getCountPart(statsItem, maxima, ignored)))
		}
	}

	if ui.showMtime {
		row += numberPrefix()
		row += fmt.Sprintf(
			"%s "+defaultColor,
			statsItem.GetMtime().Format("2006-01-02 15:04:05"),
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

	return row
}

func (ui *UI) formatFileRow(item fs.Item, maxima rowMaxima, marked, ignored bool) string {
	row := ui.formatColumns(item, maxima, marked, ignored)

	// Display symlink name in cyan/aqua (like ls --color) and target
	if name := ui.formatItemName(item, marked, ignored); name != "" {
		return row + name
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

// formatItemName returns formatted name for special item types (e.g. symlinks).
// Returns empty string if the item has no special formatting.
func (ui *UI) formatItemName(item fs.Item, marked, ignored bool) string {
	if !ui.showSymlinkTarget {
		return ""
	}
	si, ok := item.(fs.SymlinkItem)
	if !ok {
		return ""
	}
	target := si.GetSymlinkTarget()
	if target == "" {
		return ""
	}

	var name string
	if ui.UseColors && !marked && !ignored {
		name = "[aqua::b]" + tview.Escape(item.GetName())
	} else {
		name = tview.Escape(item.GetName())
	}
	return name + defaultColor + " -> " + tview.Escape(target)
}

// formatCollapsedRow formats a collapsed directory path for display
func (ui *UI) formatCollapsedRow(
	collapsedPath *CollapsedPath, maxima rowMaxima, marked, ignored bool,
) string {
	// Use the deepest directory's stats for display
	row := ui.formatColumns(collapsedPath.DeepestDir, maxima, marked, ignored)

	// Always display as directory with special formatting for collapsed path
	if ui.UseColors && !marked && !ignored {
		row += fmt.Sprintf("[%s::b]/", ui.resultRow.DirectoryColor)
	} else {
		row += defaultColorBold + "/"
	}

	// Display the collapsed path (e.g., "a/b/c")
	row += tview.Escape(collapsedPath.DisplayName)
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

	if formatted, ok := ui.FormatBlockSize(size); ok {
		return formatted + color
	}
	if ui.UseSIPrefix {
		return formatWithDecPrefix(size, color)
	}
	return formatWithBinPrefix(float64(size), color)
}

func (ui *UI) formatCount(count int64) string {
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
