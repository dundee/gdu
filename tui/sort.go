package tui

import (
	"sort"

	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// SetDefaultSorting sets the default sorting
func (ui *UI) SetDefaultSorting(by, order string) {
	if by != "" {
		ui.defaultSortBy = by
	}
	if order == "asc" || order == "desc" {
		ui.defaultSortOrder = order
	}
}

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
				sort.Sort(sort.Reverse(fs.ByApparentSize(ui.currentDir.GetFiles())))
			} else {
				sort.Sort(fs.ByApparentSize(ui.currentDir.GetFiles()))
			}
		} else {
			if ui.sortOrder == "desc" {
				sort.Sort(sort.Reverse(ui.currentDir.GetFiles()))
			} else {
				sort.Sort(ui.currentDir.GetFiles())
			}
		}
	}
	if ui.sortBy == "itemCount" {
		if ui.sortOrder == "desc" {
			sort.Sort(sort.Reverse(fs.ByItemCount(ui.currentDir.GetFiles())))
		} else {
			sort.Sort(fs.ByItemCount(ui.currentDir.GetFiles()))
		}
	}
	if ui.sortBy == "name" {
		if ui.sortOrder == "desc" {
			sort.Sort(sort.Reverse(fs.ByName(ui.currentDir.GetFiles())))
		} else {
			sort.Sort(fs.ByName(ui.currentDir.GetFiles()))
		}
	}
	if ui.sortBy == "mtime" {
		if ui.sortOrder == "desc" {
			sort.Sort(sort.Reverse(fs.ByMtime(ui.currentDir.GetFiles())))
		} else {
			sort.Sort(fs.ByMtime(ui.currentDir.GetFiles()))
		}
	}
}

func (ui *UI) sortDevices() {
	if ui.sortBy == "size" {
		if ui.sortOrder == "desc" {
			sort.Sort(sort.Reverse(device.ByUsedSize(ui.devices)))
		} else {
			sort.Sort(device.ByUsedSize(ui.devices))
		}
	}
	if ui.sortBy == "name" {
		if ui.sortOrder == "desc" {
			sort.Sort(sort.Reverse(device.ByName(ui.devices)))
		} else {
			sort.Sort(device.ByName(ui.devices))
		}
	}
}
