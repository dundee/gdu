package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/trace"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/pkg/remove"
)

func main() {
	inParallel := flag.Bool("parallel", false, "Delete in parallel")
	tracing := flag.Bool("trace", false, "Collect CPU trace")
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: [--parallel] directory_to_delete")
		return
	}

	if info, err := os.Stat(args[0]); os.IsNotExist(err) {
		fmt.Println("Path does not exist")
		return
	} else if !info.IsDir() {
		fmt.Println("Path is not directory")
		return
	}
	path, err := filepath.Abs(args[0])
	if err != nil {
		fmt.Println("Cannot convert to absolute: ", err.Error())
	}

	deleter := remove.RemoveItemFromDir
	if *inParallel {
		deleter = remove.RemoveItemFromDirParallel
	}

	if *tracing {
		f, err := os.Create("trace.out")
		if err != nil {
			fmt.Println("Trace file cannot be created: ", err.Error())
			return
		}
		trace.Start(f)
	}

	err = deleter(NewDeletedItem(""), NewDeletedItem(path))
	if err != nil {
		fmt.Println("Failed with: ", err.Error())
	}
	trace.Stop()
}

func NewDeletedItem(path string) fs.Item {
	return &DeletedItem{
		Path: path,
	}
}

type DeletedItem struct {
	Path string
}

func (p *DeletedItem) GetPath() string {
	return p.Path
}
func (p *DeletedItem) GetFilesLocked() fs.Files {
	var items []fs.Item
	entries, err := os.ReadDir(p.Path)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		path := filepath.Join(p.Path, entry.Name())
		items = append(items, NewDeletedItem(path))
	}
	return items
}
func (p *DeletedItem) IsDir() bool             { return true }
func (f *DeletedItem) RemoveFile(item fs.Item) {}

func (p *DeletedItem) GetName() string                                  { panic("must not be called") }
func (p *DeletedItem) GetFlag() rune                                    { panic("must not be called") }
func (p *DeletedItem) GetSize() int64                                   { panic("must not be called") }
func (p *DeletedItem) GetType() string                                  { panic("must not be called") }
func (p *DeletedItem) GetUsage() int64                                  { panic("must not be called") }
func (p *DeletedItem) GetMtime() time.Time                              { panic("must not be called") }
func (p *DeletedItem) GetItemCount() int                                { panic("must not be called") }
func (p *DeletedItem) GetParent() fs.Item                               { panic("must not be called") }
func (p *DeletedItem) SetParent(fs.Item)                                { panic("must not be called") }
func (p *DeletedItem) GetMultiLinkedInode() uint64                      { panic("must not be called") }
func (p *DeletedItem) EncodeJSON(writer io.Writer, topLevel bool) error { panic("must not be called") }
func (p *DeletedItem) UpdateStats(linkedItems fs.HardLinkedItems)       { panic("must not be called") }
func (p *DeletedItem) AddFile(fs.Item)                                  { panic("must not be called") }
func (p *DeletedItem) GetFiles() fs.Files                               { panic("must not be called") }
func (p *DeletedItem) RLock() func()                                    { panic("must not be called") }
func (p *DeletedItem) SetFiles(fs.Files)                                { panic("must not be called") }
func (p *DeletedItem) GetItemStats(
	linkedItems fs.HardLinkedItems,
) (int, int64, int64) {
	panic("must not be called")
}
