package common

import (
	"path/filepath"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/pkg/device"
)

// getFilesystemID is a variable so that tests can replace it
var getFilesystemID = device.GetFilesystemID

const filesystemIDCacheSize = 1024

type filesystemIDEntry struct {
	path string
	id   uint64
	ok   bool
}

// filesystemIDCache remembers the filesystem IDs of recently checked paths.
// Every dir is looked up once when its parent is listed and then again as the
// parent of each of its subdirs, so caching saves most of the stat calls.
// The cache is direct-mapped with a fixed size, which keeps memory bounded
// and lets parallel analyzers share it without locking.
type filesystemIDCache struct {
	slots [filesystemIDCacheSize]atomic.Pointer[filesystemIDEntry]
}

func newFilesystemIDCache() *filesystemIDCache {
	return &filesystemIDCache{}
}

// fnv1a returns the 64-bit FNV-1a hash of the string
func fnv1a(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func (c *filesystemIDCache) filesystemID(path string) (uint64, bool) {
	slot := &c.slots[fnv1a(path)%filesystemIDCacheSize]
	if entry := slot.Load(); entry != nil && entry.path == path {
		return entry.id, entry.ok
	}
	id, ok := getFilesystemID(path)
	slot.Store(&filesystemIDEntry{path: path, id: id, ok: ok})
	return id, ok
}

// isOnOtherFilesystem returns true if the dir lives on another filesystem
// (device) than its parent dir. This also catches boundaries which are not
// listed as mount points, e.g. btrfs subvolumes and snapshots.
// If either device cannot be determined, the dir is not ignored.
func (c *filesystemIDCache) isOnOtherFilesystem(path string) bool {
	dev, ok := c.filesystemID(path)
	if !ok {
		return false
	}
	parentDev, ok := c.filesystemID(filepath.Dir(path))
	if !ok {
		return false
	}
	shouldIgnore := dev != parentDev
	if shouldIgnore {
		log.Printf("Directory %s ignored (other filesystem)", path)
	}
	return shouldIgnore
}
