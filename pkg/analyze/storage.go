package analyze

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dundee/gdu/v5/internal/common"
)

// storedFormat is the version of the records in the database. A database
// holding records of another version has to be scanned again.
const storedFormat = 1

// errStoredFormat is returned for a record of another format.
var errStoredFormat = errors.New("the database was written by another version of gdu, scan again to replace it")

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
	Mtime                            time.Time
	Entries                          []storedEntry
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

// Storage represents a badger storage
//
// All access to db is serialized by m. Open is reference counted: badger takes
// an exclusive lock on the storage directory, so a second concurrent
// badger.Open on the same path would panic, and an inner caller's closer must
// not tear the database out from under an outer caller that is still using it.
type Storage struct {
	db          *badger.DB
	storagePath string
	m           sync.RWMutex
	counter     atomic.Int64
	openCount   int
}

// NewStorage returns new instance of badger storage
func NewStorage(storagePath string) *Storage {
	return &Storage{storagePath: storagePath}
}

// IsOpen returns true if badger DB is open
func (s *Storage) IsOpen() bool {
	s.m.RLock()
	defer s.m.RUnlock()
	return s.db != nil
}

// Open opens badger DB and returns a function closing it again.
//
// Opening is reference counted, so nested and concurrent callers all get a
// usable database and only the last returned closer actually closes it. Each
// closer is idempotent; calling it more than once is a no-op.
func (s *Storage) Open() func() {
	s.m.Lock()
	defer s.m.Unlock()

	if s.db == nil {
		s.openLocked()
	}
	s.openCount++

	var once sync.Once
	return func() {
		once.Do(s.release)
	}
}

// release drops one reference taken by Open, closing the database once the
// last one is gone.
func (s *Storage) release() {
	s.m.Lock()
	defer s.m.Unlock()

	if s.openCount == 0 {
		return
	}
	s.openCount--
	if s.openCount > 0 || s.db == nil {
		return
	}
	s.db.Close()
	s.db = nil
}

// Sizes of badger's in-memory structures. Its defaults (five 64 MiB memtables
// and a 256 MiB block cache) are meant for a long-running database server and
// made a scan into badger use several times the memory of a scan without it,
// although keeping memory low is the point of --db. Smaller ones cost no
// speed: a scan writes every directory once, and reads go through gdu's own
// tree. The memtable cannot be much smaller, as badger refuses to open when
// its largest batch, 15% of a memtable, is below the 1 MiB value threshold.
const (
	badgerMemTableSize   = 16 << 20
	badgerNumMemtables   = 2
	badgerBlockCacheSize = 16 << 20
)

// openLocked opens the badger DB. The caller must hold m for writing.
func (s *Storage) openLocked() {
	options := badger.DefaultOptions(s.storagePath)
	options.Logger = nil
	options.MemTableSize = badgerMemTableSize
	options.NumMemtables = badgerNumMemtables
	options.BlockCacheSize = badgerBlockCacheSize

	if !common.Is64Bit {
		// For 32-bit systems, we need to set ValueLogFileSize to a smaller value to
		// avoid "cannot allocate memory while mmapping" error
		options.ValueLogFileSize = (1<<30 - 1) / 2
	}

	db, err := badger.Open(options)
	if err != nil {
		panic(err)
	}
	s.db = db
}

// storeRecord stores the record of the directory at path.
func (s *Storage) storeRecord(path string, record *storedRecord) error {
	record.Format = storedFormat
	var value bytes.Buffer
	if err := gob.NewEncoder(&value).Encode(record); err != nil {
		return fmt.Errorf("encoding the record of %s: %w", path, err)
	}

	s.checkCount()
	s.m.RLock()
	defer s.m.RUnlock()
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(path), value.Bytes())
	})
}

// loadRecord returns the record of the directory at path.
func (s *Storage) loadRecord(path string) (*storedRecord, error) {
	s.checkCount()
	s.m.RLock()
	defer s.m.RUnlock()

	record := &storedRecord{}
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(path))
		if err != nil {
			return fmt.Errorf("reading stored value for path %s: %w", path, err)
		}
		return item.Value(func(value []byte) error {
			if gob.NewDecoder(bytes.NewReader(value)).Decode(record) != nil || record.Format != storedFormat {
				return errStoredFormat
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetDirForPath returns the stored directory at path, loaded.
func (s *Storage) GetDirForPath(path string) (*StoredDir, error) {
	dir := newStoredDir(s, path, nil)
	if err := dir.load(); err != nil {
		return nil, err
	}
	return dir, nil
}

func (s *Storage) checkCount() {
	s.counter.Add(1)
	if s.counter.Load() >= 10000 {
		s.m.Lock()
		defer s.m.Unlock()
		if s.counter.Load() < 10000 {
			// another goroutine already recycled the database
			return
		}
		s.counter.Store(0)
		if s.db != nil {
			s.db.Close()
		}
		s.openLocked()
	}
}
