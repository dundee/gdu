package analyze

import (
	"github.com/dundee/gdu/v5/pkg/fs"
)

// TopList is a list of top largest files
type TopList struct {
	Items   fs.Files
	Count   int
	MinSize int64
	SortBy  fs.SortBy
}

// NewTopList creates new TopList ranking files by sortBy
func NewTopList(count int, sortBy fs.SortBy) *TopList {
	return &TopList{Count: count, SortBy: sortBy}
}

// rankedSize returns the value of the ranking metric of the item, so that the
// list is built from the same number the caller orders and displays by.
func (tl *TopList) rankedSize(item fs.Item) int64 {
	if tl.SortBy == fs.SortByApparentSize {
		return item.GetSize()
	}
	return item.GetUsage()
}

// Add adds file to the list
func (tl *TopList) Add(file fs.Item) {
	if tl.rankedSize(file) > tl.MinSize || len(tl.Items) < tl.Count {
		tl.Items = append(tl.Items, file)
		sortFiles(tl.Items, tl.SortBy, fs.SortAsc)
		if len(tl.Items) > tl.Count {
			tl.Items = tl.Items[1:]
		}
		tl.MinSize = tl.rankedSize(tl.Items[0])
	}
}

// CollectTopFiles returns the count largest files in dir, ranked by sortBy
func CollectTopFiles(dir fs.Item, count int, sortBy fs.SortBy) fs.Files {
	topList := NewTopList(count, sortBy)
	walkDir(dir, topList)
	sortFiles(topList.Items, sortBy, fs.SortDesc)
	return topList.Items
}

func walkDir(dir fs.Item, topList *TopList) {
	for item := range dir.GetFiles(fs.SortBySize, fs.SortDesc) {
		if item.IsDir() {
			walkDir(item, topList)
		} else {
			topList.Add(item)
		}
	}
}
