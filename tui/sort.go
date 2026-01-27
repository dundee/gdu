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

// getSortParams returns the current sort parameters as fs.SortBy and fs.SortOrder
func (ui *UI) getSortParams() (fs.SortBy, fs.SortOrder) {
	var sortBy fs.SortBy
	switch ui.sortBy {
	case nameSortKey:
		sortBy = fs.SortByName
	case itemCountSortKey:
		sortBy = fs.SortByItemCount
	case mtimeSortKey:
		sortBy = fs.SortByMtime
	case sizeSortKey:
		if ui.ShowApparentSize {
			sortBy = fs.SortByApparentSize
		} else {
			sortBy = fs.SortBySize
		}
	default:
		sortBy = fs.SortBySize
	}

	sortOrder := fs.SortAsc
	if ui.sortOrder == descOrder {
		sortOrder = fs.SortDesc
	}

	return sortBy, sortOrder
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
