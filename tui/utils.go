package tui

import (
	"path/filepath"

	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/rivo/tview"
)

var (
	barFullRune  = "\u2588"
	barPartRunes = map[int]string{
		0: " ",
		1: "\u258F",
		2: "\u258E",
		3: "\u258D",
		4: "\u258C",
		5: "\u258B",
		6: "\u258A",
		7: "\u2589",
	}
)

func getDeviceUsagePart(item *device.Device, useOld bool) string {
	part := int(float64(item.Size-item.Free) / float64(item.Size) * 100.0)
	if useOld {
		return getUsageGraphOld(part)
	}
	return getUsageGraph(part)
}

func getUsageGraph(part int) string {
	graph := " "
	whole := part / 10
	for i := 0; i < whole; i++ {
		graph += barFullRune
	}
	partWidth := (part % 10) * 8 / 10
	if part < 100 {
		graph += barPartRunes[partWidth]
	}

	for i := 0; i < 10-whole-1; i++ {
		graph += " "
	}

	graph += "\u258F"
	return graph
}

func getUsageGraphOld(part int) string {
	part /= 10
	graph := "["
	for i := 0; i < 10; i++ {
		if part > i {
			graph += "#"
		} else {
			graph += " "
		}
	}
	graph += "]"
	return graph
}

func modal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}

// CollapsedPath represents a directory chain that can be collapsed into a single display entry.
// For example, if directory "a" contains only directory "b", and "b" contains only "c",
// this represents the collapsed path "a/b/c" that allows direct navigation to the deepest directory.
type CollapsedPath struct {
	DisplayName string   // The display name shown in the UI (e.g., "a/b/c")
	DeepestDir  fs.Item  // The actual deepest directory item
	Segments    []string // Individual path segments of the collapsed chain
}

// findCollapsiblePath checks if the given directory item has a single subdirectory chain
// and returns a CollapsedPath if it can be collapsed
func findCollapsiblePath(item fs.Item) *CollapsedPath {
	if item == nil || !item.IsDir() {
		return nil
	}

	var segments []string
	current := item

	for {
		files := current.GetFiles()

		// Count directories and files separately
		var subdirs []fs.Item
		var fileCount int
		for _, file := range files {
			if file.IsDir() {
				subdirs = append(subdirs, file)
			} else {
				fileCount++
			}
		}

		// Only collapse if there's exactly one subdirectory AND no files
		if len(subdirs) != 1 || fileCount > 0 {
			break
		}

		// Add this segment to the path
		segments = append(segments, subdirs[0].GetName())
		current = subdirs[0]
	}

	// Only create collapsed path if we have at least one collapsible segment
	if len(segments) == 0 {
		return nil
	}

	return &CollapsedPath{
		DisplayName: filepath.Join(segments...),
		DeepestDir:  current,
		Segments:    segments,
	}
}

// findCollapsedParent checks if the current directory is the deepest directory
// in a collapsed path, and returns the appropriate parent to navigate to
func findCollapsedParent(currentDir fs.Item) fs.Item {
	if currentDir == nil {
		return nil
	}
	if currentDir.GetParent() == nil {
		return nil
	}

	// Check if current directory is part of a single-child chain going up
	current := currentDir
	var chainParent fs.Item

	// Walk up the parent chain
	for current.GetParent() != nil {
		parent := current.GetParent()

		// Count subdirectories in parent
		var subdirCount int
		for _, file := range parent.GetFiles() {
			if file.IsDir() {
				subdirCount++
			}
		}

		// If parent has more than one subdirectory, this is where the collapsed chain starts
		if subdirCount > 1 {
			chainParent = parent
			break
		}

		// Move up the chain
		current = parent
	}

	// If we found a chain parent (meaning current dir is part of a collapsed path),
	// return it, otherwise return the normal parent
	if chainParent != nil {
		return chainParent
	}

	return currentDir.GetParent()
}
