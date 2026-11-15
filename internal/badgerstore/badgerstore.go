// Package badgerstore keeps the records of a stored analysis (--db x.badger)
// in a badger database. It implements analyze.RecordStore.
package badgerstore

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v4"
)

// is64Bit is true on 64-bit systems.
const is64Bit = (^uint(0) >> 63) == 1

// ErrClosed is returned when the store is used after Close.
var ErrClosed = errors.New("the badger database is closed")

// Store is a badger database of records, keyed by bytes.
//
// The database is opened on first use and stays open until Close: opening
// it costs about 11 ms on a database of a million items, which browsing paid
// for every directory when the database was opened per operation. Badger
// locks the storage directory, so a second Store of the same directory, in
// this process or another one, cannot open it while this one is open.
type Store struct {
	storagePath string
	// m guards db and closed
	m      sync.RWMutex
	db     *badger.DB
	closed bool
}

// New returns a store of the badger database in the directory storagePath.
// The database is opened on first use.
func New(storagePath string) *Store {
	return &Store{storagePath: storagePath}
}

// Path returns the directory of the database.
func (s *Store) Path() string {
	return s.storagePath
}

// IsOpen returns true if badger DB is open
func (s *Store) IsOpen() bool {
	s.m.RLock()
	defer s.m.RUnlock()
	return s.db != nil
}

// Close closes the database. The store cannot be used afterwards.
func (s *Store) Close() error {
	s.m.Lock()
	defer s.m.Unlock()
	s.closed = true
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// use calls f with the database, opening it on first use. Close waits for
// the calls running.
func (s *Store) use(f func(db *badger.DB) error) error {
	s.m.RLock()
	if s.db == nil {
		s.m.RUnlock()
		if err := s.open(); err != nil {
			return err
		}
		s.m.RLock()
	}
	defer s.m.RUnlock()
	if s.db == nil {
		return ErrClosed
	}
	return f(s.db)
}

// open opens the database, unless it is open already or closed.
func (s *Store) open() error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.closed {
		return ErrClosed
	}
	if s.db != nil {
		return nil
	}
	db, err := openBadger(s.storagePath)
	if err != nil {
		return fmt.Errorf("opening the badger database %s: %w", s.storagePath, err)
	}
	s.db = db
	return nil
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

// openBadger opens the badger database in the directory storagePath.
func openBadger(storagePath string) (*badger.DB, error) {
	options := badger.DefaultOptions(storagePath)
	options.Logger = nil
	options.MemTableSize = badgerMemTableSize
	options.NumMemtables = badgerNumMemtables
	options.BlockCacheSize = badgerBlockCacheSize

	if !is64Bit {
		// For 32-bit systems, we need to set ValueLogFileSize to a smaller value to
		// avoid "cannot allocate memory while mmapping" error
		options.ValueLogFileSize = (1<<30 - 1) / 2
	}

	return badger.Open(options)
}

// Put stores value under key.
func (s *Store) Put(key, value []byte) error {
	return s.use(func(db *badger.DB) error {
		return db.Update(func(txn *badger.Txn) error {
			return txn.Set(key, value)
		})
	})
}

// Get returns the value stored under key; found is false when there is
// none.
func (s *Store) Get(key []byte) (value []byte, found bool, err error) {
	err = s.use(func(db *badger.DB) error {
		return db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(key)
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("reading stored value for path %s: %w", key, err)
			}
			found = true
			value, err = item.ValueCopy(nil)
			return err
		})
	})
	return value, found, err
}
