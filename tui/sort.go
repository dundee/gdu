package tui

import (
	"sort"

	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
)

const (
	nameSortKey      = "name"
	sizeSortKey      = "size"
	itemCountSortKey = "itemCount"
	mtimeSortKey     = "mtime"

	ascOrder  = "asc"
	descOrder = "desc"
)

// SetDefaultSorting sets the default sorting
func (ui *UI) SetDefaultSorting(by, order string) {
	if by != "" {
		ui.defaultSortBy = by
	}
	if order == ascOrder || order == descOrder {
		ui.defaultSortOrder = order
	}
}

func (ui *UI) setSorting(newOrder string) {
	ui.markedRows = make(map[int]struct{})

	if newOrder == ui.sortBy {
		if ui.sortOrder == ascOrder {
			ui.sortOrder = descOrder
		} else {
			ui.sortOrder = ascOrder
		}
	} else {
		ui.sortBy = newOrder
		ui.sortOrder = ascOrder
	}

	if ui.currentDir != nil {
		ui.showDir()
	} else if ui.devices != nil && (newOrder == sizeSortKey || newOrder == nameSortKey) {
		ui.showDevices()
	}
}

func (ui *UI) sortItems() {
	if ui.sortBy == sizeSortKey {
		if ui.ShowApparentSize {
			if ui.sortOrder == descOrder {
				sort.Sort(sort.Reverse(fs.ByApparentSize(ui.currentDir.GetFiles())))
			} else {
				sort.Sort(fs.ByApparentSize(ui.currentDir.GetFiles()))
			}
		} else {
			if ui.sortOrder == descOrder {
				sort.Sort(sort.Reverse(ui.currentDir.GetFiles()))
			} else {
				sort.Sort(ui.currentDir.GetFiles())
			}
		}
	}
	if ui.sortBy == itemCountSortKey {
		if ui.sortOrder == descOrder {
			sort.Sort(sort.Reverse(fs.ByItemCount(ui.currentDir.GetFiles())))
		} else {
			sort.Sort(fs.ByItemCount(ui.currentDir.GetFiles()))
		}
	}
	if ui.sortBy == nameSortKey {
		if ui.sortOrder == descOrder {
			sort.Sort(sort.Reverse(fs.ByName(ui.currentDir.GetFiles())))
		} else {
			sort.Sort(fs.ByName(ui.currentDir.GetFiles()))
		}
	}
	if ui.sortBy == mtimeSortKey {
		if ui.sortOrder == descOrder {
			sort.Sort(sort.Reverse(fs.ByMtime(ui.currentDir.GetFiles())))
		} else {
			sort.Sort(fs.ByMtime(ui.currentDir.GetFiles()))
		}
	}
}

func (ui *UI) sortDevices() {
	if ui.sortBy == sizeSortKey {
		if ui.sortOrder == descOrder {
			sort.Sort(sort.Reverse(device.ByUsedSize(ui.devices)))
		} else {
			sort.Sort(device.ByUsedSize(ui.devices))
		}
	}
	if ui.sortBy == nameSortKey {
		if ui.sortOrder == descOrder {
			sort.Sort(sort.Reverse(device.ByName(ui.devices)))
		} else {
			sort.Sort(device.ByName(ui.devices))
		}
	}
}
