package analyze

import (
	"fmt"
	"sync/atomic"
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
// Package github.com/dundee/gdu/v6/pkg/analyze/badger provides one keeping
// them in a badger database.
//
// Put and Get are called concurrently. The store opens on first use and
// stays open until Close, after which it fails.
type RecordStore interface {
	// Close closes the store.
	Close() error
	// Put stores value under key.
	Put(key, value []byte) error
	// Get returns the value stored under key; found is false when there is
	// none.
	Get(key []byte) (value []byte, found bool, err error)
}

// records reads and writes the records of directories in a RecordStore.
type records struct {
	RecordStore
	// failed is set when storing a record fails
	failed *atomic.Bool
}

func newRecords(store RecordStore) records {
	return records{RecordStore: store, failed: &atomic.Bool{}}
}

// storeRecord stores the record of the directory at path.
func (s records) storeRecord(path string, record *storedRecord) error {
	record.Format = storedFormat
	err := s.Put([]byte(path), record.marshal())
	if err != nil {
		s.failed.Store(true)
	}
	return err
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

// infoKey is the key of the record of the analysis the store holds. No path
// starts with a NUL.
var infoKey = []byte("\x00analysis")

func (s records) writeInfo(info analysisInfo) error {
	value, err := info.marshal()
	if err != nil {
		return err
	}
	return s.Put(infoKey, value)
}

func (s records) holdsUnrecorded(path string) (bool, error) {
	_, found, err := s.Get([]byte(path))
	return found, err
}

func (s records) readInfo() (info analysisInfo, ok bool, err error) {
	value, found, err := s.Get(infoKey)
	if err != nil || !found {
		return info, false, err
	}
	info, err = unmarshalInfo(value)
	return info, err == nil, err
}
