package analyze

import (
	"sort"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// TopList is a list of top largest files
type TopList struct {
	Items   fs.Files
	Count   int
	MinSize int64
}

// NewTopList creates new TopList
func NewTopList(count int) *TopList {
	return &TopList{Count: count}
}

// Add adds file to the list
func (tl *TopList) Add(file fs.Item) {
	if file.GetSize() > tl.MinSize || len(tl.Items) < tl.Count {
		tl.Items = append(tl.Items, file)
		sort.Sort(fs.ByApparentSize(tl.Items))
		if len(tl.Items) > tl.Count {
			tl.Items = tl.Items[1:]
		}
		tl.MinSize = tl.Items[0].GetSize()
	}
}

func CollectTopFiles(dir fs.Item, count int) fs.Files {
	topList := NewTopList(count)
	walkDir(dir, topList)
	sort.Sort(sort.Reverse(fs.ByApparentSize(topList.Items)))
	return topList.Items
}

func walkDir(dir fs.Item, topList *TopList) {
	for _, item := range dir.GetFiles() {
		if item.IsDir() {
			walkDir(item, topList)
		} else {
			topList.Add(item)
		}
	}
}
