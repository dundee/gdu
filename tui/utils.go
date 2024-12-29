package tui

import (
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/rivo/tview"
)

var (
	barFullRune  = "\u2588"
	barPartRunes = map[int]string{
		0: " ",
		1: "\u258F",
		2: "\u258E",
		3: "\u258D",
		4: "\u258C",
		5: "\u258B",
		6: "\u258A",
		7: "\u2589",
	}
)

func getDeviceUsagePart(item *device.Device, useOld bool) string {
	part := int(float64(item.Size-item.Free) / float64(item.Size) * 100.0)
	if useOld {
		return getUsageGraphOld(part)
	}
	return getUsageGraph(part)
}

func getUsageGraph(part int) string {
	graph := " "
	whole := part / 10
	for i := 0; i < whole; i++ {
		graph += barFullRune
	}
	partWidth := (part % 10) * 8 / 10
	if part < 100 {
		graph += barPartRunes[partWidth]
	}

	for i := 0; i < 10-whole-1; i++ {
		graph += " "
	}

	graph += "\u258F"
	return graph
}

func getUsageGraphOld(part int) string {
	part /= 10
	graph := "["
	for i := 0; i < 10; i++ {
		if part > i {
			graph += "#"
		} else {
			graph += " "
		}
	}
	graph += "]"
	return graph
}

func modal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}
