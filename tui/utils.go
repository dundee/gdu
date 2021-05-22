package tui

import (
	"github.com/dundee/gdu/v5/pkg/device"
)

func getDeviceUsagePart(item *device.Device) string {
	part := int(float64(item.Size-item.Free) / float64(item.Size) * 10.0)
	row := "["
	for i := 0; i < 10; i++ {
		if part > i {
			row += "#"
		} else {
			row += " "
		}
	}
	row += "]"
	return row
}

func getUsageGraph(part int) string {
	graph := " ["
	for i := 0; i < 10; i++ {
		if part > i {
			graph += "#"
		} else {
			graph += " "
		}
	}
	graph += "] "
	return graph
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
