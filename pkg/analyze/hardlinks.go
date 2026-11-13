package analyze

import "sync"

// inodeKey identifies a file with several hard links. Inode numbers are
// unique only within a filesystem, so the device is part of the key. It is
// zero for a file with a single link, and on platforms without inodes.
type inodeKey struct {
	dev, ino uint64
}

// linkTracker records the hard-linked files met by a scan that totals as it
// goes, so that each is counted once however many links lead to it. It is
// safe for concurrent use.
type linkTracker struct {
	seen sync.Map
}

// claim reports whether the file is met for the first time.
func (t *linkTracker) claim(key inodeKey) bool {
	_, seen := t.seen.LoadOrStore(key, struct{}{})
	return !seen
}

// reset forgets every file met, for a new scan.
func (t *linkTracker) reset() {
	t.seen.Clear()
}
