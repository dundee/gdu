package analyze

import (
	"fmt"
	"time"
)

// storedFormat is the version of the records in the database. A database
// holding records of another version has to be scanned again.
const storedFormat = 2

// storedRecord is what the database holds for a directory, under its path.
// It holds no references to other items, so a stored tree never has to be
// encoded as a graph.
type storedRecord struct {
	Format int
	// Type is the GetType of a directory inside a browsed archive, and empty
	// for a directory on disk.
	Type                             string
	Flag                             rune
	Size, Usage, OwnUsage, ItemCount int64
	// Dev is the device of the directory
	Dev     uint64
	Mtime   time.Time
	Entries []storedEntry
}

// storedEntry is an entry of a stored directory. An entry for a directory
// only names it; the directory's own record holds the rest.
type storedEntry struct {
	Name                     string
	Dir                      bool
	Flag                     rune
	Size, Usage, SharedUsage int64
	Mtime                    time.Time
	Mli                      uint64
	Symlink                  string
}

// RecordStore keeps the records of a stored analysis: bytes under keys.
// Package github.com/dundee/gdu/v5/pkg/analyze/badger provides one keeping
// them in a badger database.
//
// Put and Get are only called while the store is open. Open may be called
// again, also concurrently, before the function it returned is called: only
// the last of those functions closes the store.
type RecordStore interface {
	// Open opens the store and returns a function closing it again.
	Open() (closeFn func())
	// Put stores value under key.
	Put(key, value []byte) error
	// Get returns the value stored under key; found is false when there is
	// none.
	Get(key []byte) (value []byte, found bool, err error)
}

// records reads and writes the records of directories in a RecordStore.
type records struct {
	RecordStore
}

// storeRecord stores the record of the directory at path.
func (s records) storeRecord(path string, record *storedRecord) error {
	record.Format = storedFormat
	return s.Put([]byte(path), record.marshal())
}

// loadRecord returns the record of the directory at path.
func (s records) loadRecord(path string) (*storedRecord, error) {
	value, found, err := s.Get([]byte(path))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%w of %s", ErrNoAnalysis, path)
	}
	record := &storedRecord{}
	if record.unmarshal(value) != nil || record.Format != storedFormat {
		return nil, ErrStoredFormat
	}
	return record, nil
}

// dirForPath returns the stored directory at path, loaded.
func (s records) dirForPath(path string) (*StoredDir, error) {
	dir := newStoredDir(s, path, nil)
	if err := dir.load(); err != nil {
		return nil, err
	}
	return dir, nil
}
