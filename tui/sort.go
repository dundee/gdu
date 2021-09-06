package tui

import (
	"sort"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/device"
)

func (ui *UI) setSorting(newOrder string) {
	if newOrder == ui.sortBy {
		if ui.sortOrder == "asc" {
			ui.sortOrder = "desc"
		} else {
			ui.sortOrder = "asc"
		}
	} else {
		ui.sortBy = newOrder
		ui.sortOrder = "asc"
	}

	if ui.currentDir != nil {
		ui.showDir()
	} else if ui.devices != nil && (newOrder == "size" || newOrder == "name") {
		ui.showDevices()
	}
}

func (ui *UI) sortItems() {
	if ui.sortBy == "size" {
		if ui.ShowApparentSize {
			if ui.sortOrder == "desc" {
				sort.Sort(analyze.ByApparentSize(ui.currentDir.Files))
			} else {
				sort.Sort(sort.Reverse(analyze.ByApparentSize(ui.currentDir.Files)))
			}
		} else {
			if ui.sortOrder == "desc" {
				sort.Sort(ui.currentDir.Files)
			} else {
				sort.Sort(sort.Reverse(ui.currentDir.Files))
			}
		}
	}
	if ui.sortBy == "itemCount" {
		if ui.sortOrder == "desc" {
			sort.Sort(analyze.ByItemCount(ui.currentDir.Files))
		} else {
			sort.Sort(sort.Reverse(analyze.ByItemCount(ui.currentDir.Files)))
		}
	}
	if ui.sortBy == "name" {
		if ui.sortOrder == "desc" {
			sort.Sort(analyze.ByName(ui.currentDir.Files))
		} else {
			sort.Sort(sort.Reverse(analyze.ByName(ui.currentDir.Files)))
		}
	}
	if ui.sortBy == "mtime" {
		if ui.sortOrder == "desc" {
			sort.Sort(analyze.ByMtime(ui.currentDir.Files))
		} else {
			sort.Sort(sort.Reverse(analyze.ByMtime(ui.currentDir.Files)))
		}
	}
}

func (ui *UI) sortDevices() {
	if ui.sortBy == "size" {
		if ui.sortOrder == "desc" {
			sort.Sort(device.ByUsedSize(ui.devices))
		} else {
			sort.Sort(sort.Reverse(device.ByUsedSize(ui.devices)))
		}
	}
	if ui.sortBy == "name" {
		if ui.sortOrder == "desc" {
			sort.Sort(device.ByName(ui.devices))
		} else {
			sort.Sort(sort.Reverse(device.ByName(ui.devices)))
		}
	}
}
