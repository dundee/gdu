package analyze

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// TestPartialTreeLockedAccessors exercises the preview contract against the
// same mutations a scan performs while constructing and updating directories.
// The fixed round barriers make the stress repeatable without timing sleeps.
func TestPartialTreeLockedAccessors(t *testing.T) {
	const rounds = 256
	const mutationsPerRound = 8

	root := &Dir{
		File:  &File{Name: "root"},
		Files: make(fs.Files, 0, rounds),
	}
	analyzer := CreateAnalyzer()
	analyzer.setCurrentDir(root)

	for round := range rounds {
		start := make(chan struct{})
		var workers sync.WaitGroup
		workers.Add(2)

		go func(round int) {
			defer workers.Done()
			<-start

			child := &Dir{
				File: &File{
					Name:   fmt.Sprintf("dir-%03d", round),
					Mtime:  time.Unix(int64(round), 0),
					Parent: root,
				},
				Files: make(fs.Files, 0, mutationsPerRound),
			}
			root.AddFile(child)
			for mutation := range mutationsPerRound {
				child.AddFile(&File{
					Name:   fmt.Sprintf("file-%03d-%02d", round, mutation),
					Size:   int64(mutation + 1),
					Usage:  int64(mutation + 1),
					Mli:    uint64(round + 1),
					Mtime:  time.Unix(int64(round), int64(mutation)),
					Parent: child,
				})
				child.SetFlag('!')
				child.UpdateStats(make(fs.HardLinkedItems))
				root.UpdateStats(make(fs.HardLinkedItems))
				runtime.Gosched()
			}
		}(round)

		// This models a preview-triggered stats update overlapping the analyzer
		// worker's own update traversal on the same partially built tree.
		go func() {
			defer workers.Done()
			<-start
			for range mutationsPerRound {
				root.UpdateStats(make(fs.HardLinkedItems))
				runtime.Gosched()
			}
		}()

		close(start)
		current, ok := analyzer.GetCurrentDir().(*Dir)
		if !ok {
			t.Fatal("current directory unavailable during partial scan")
		}
		readLockedTree(current)
		workers.Wait()
	}

	root.UpdateStats(make(fs.HardLinkedItems))
	if got, want := root.GetItemCount(), int64(1+rounds*(mutationsPerRound+1)); got != want {
		t.Fatalf("partial tree stats = %d, want %d", got, want)
	}
}

// readLockedTree mirrors preview traversal: every mutable directory is read
// through GetFilesLocked and its synchronized scalar accessors.
func readLockedTree(root *Dir) {
	_ = root.GetName()
	_ = root.GetFlag()
	_ = root.GetSize()
	_ = root.GetUsage()
	_ = root.GetMtime()
	_ = root.GetItemCount()
	for item := range root.GetFilesLocked(fs.SortBySize, fs.SortDesc) {
		_ = item.GetName()
		_ = item.GetFlag()
		_ = item.GetSize()
		_ = item.GetUsage()
		_ = item.GetMtime()
		_ = item.GetItemCount()
		if child, ok := item.(*Dir); ok {
			for nested := range child.GetFilesLocked(fs.SortByName, fs.SortAsc) {
				_ = nested.GetName()
				_ = nested.GetFlag()
				_ = nested.GetSize()
				_ = nested.GetUsage()
				_ = nested.GetMtime()
				_ = nested.GetItemCount()
			}
		}
	}
}
