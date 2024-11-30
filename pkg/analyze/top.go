package analyze

import (
	"sort"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// TopList is a list of top largest files
type TopList struct {
	Count   int
	Items   fs.Files
	MinSize int64
}

// NewTopList creates new TopList
func NewTopList(count int) *TopList {
	return &TopList{Count: count}
}

// Add adds file to the list
func (tl *TopList) Add(file fs.Item) {
	if len(tl.Items) < tl.Count {
		tl.Items = append(tl.Items, file)
		if file.GetSize() > tl.MinSize {
			tl.MinSize = file.GetSize()
		}
	} else if file.GetSize() > tl.MinSize {
		tl.Items = append(tl.Items, file)
		tl.MinSize = file.GetSize()
		if len(tl.Items) > tl.Count {
			sort.Sort(fs.ByApparentSize(tl.Items))
			tl.Items = tl.Items[1:]
		}
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
