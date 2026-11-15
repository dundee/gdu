package fs

import (
	"strings"
	"sync"
)

// InodeKey identifies a file with several hard links. Inode numbers are
// unique only within a filesystem, so the device is part of the key.
type InodeKey struct {
	Dev, Ino uint64
}

// HardLinks indexes the hard-linked files of a tree by their identity. It is
// built when the totals of the tree are computed, which count each file once:
// through the first of its links met, which is the first of its group here.
// It is safe for concurrent use.
type HardLinks struct {
	mu     sync.Mutex
	groups map[InodeKey]Files
}

// NewHardLinks returns an empty index.
func NewHardLinks() *HardLinks {
	return &HardLinks{groups: make(map[InodeKey]Files)}
}

// Add records item as a link to the file key, and reports whether it is the
// first, the one counted in the totals.
func (h *HardLinks) Add(key InodeKey, item Item) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.groups[key] = append(h.groups[key], item)
	return len(h.groups[key]) == 1
}

// Links returns the links to the file key, the counted one first. A nil index
// has none.
func (h *HardLinks) Links(key InodeKey) Files {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return append(Files(nil), h.groups[key]...)
}

// Remove drops the links lying at path or below it, which have been removed
// from the tree. It reports whether it held any, and whether a counted one was
// among them, and returns the links counted in their place: for each file
// that lost its counted link but has others left, the next one.
func (h *HardLinks) Remove(path, separator string) (found, removedCounted bool, nowCounted Files) {
	if h == nil {
		return false, false, nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, group := range h.groups {
		kept := group[:0:0]
		counted := false
		for i, item := range group {
			itemPath := item.GetPath()
			if itemPath == path || strings.HasPrefix(itemPath, path+separator) {
				counted = counted || i == 0
				continue
			}
			kept = append(kept, item)
		}
		if len(kept) == len(group) {
			continue
		}
		found = true
		if counted {
			removedCounted = true
			if len(kept) > 0 {
				nowCounted = append(nowCounted, kept[0])
			}
		}
		if len(kept) == 0 {
			delete(h.groups, key)
		} else {
			h.groups[key] = kept
		}
	}
	return found, removedCounted, nowCounted
}
