// Package badgerstore keeps the records of a stored analysis (--db x.badger)
// in a badger database. It implements analyze.RecordStore.
package badgerstore

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/dgraph-io/badger/v4"
)

// is64Bit is true on 64-bit systems.
const is64Bit = (^uint(0) >> 63) == 1

// reopenEvery is the number of operations after which the database is closed
// and opened again, which releases the memory badger holds on to.
const reopenEvery = 10000

// Store is a badger database of records, keyed by bytes.
//
// All access to db is serialized by m. Open is reference counted: badger takes
// an exclusive lock on the storage directory, so a second concurrent
// badger.Open on the same path would panic, and an inner caller's closer must
// not tear the database out from under an outer caller that is still using it.
type Store struct {
	db          *badger.DB
	storagePath string
	m           sync.RWMutex
	counter     atomic.Int64
	openCount   int
}

// New returns a store of the badger database in the directory storagePath.
// The database is opened by Open.
func New(storagePath string) *Store {
	return &Store{storagePath: storagePath}
}

// IsOpen returns true if badger DB is open
func (s *Store) IsOpen() bool {
	s.m.RLock()
	defer s.m.RUnlock()
	return s.db != nil
}

// Open opens badger DB and returns a function closing it again.
//
// Opening is reference counted, so nested and concurrent callers all get a
// usable database and only the last returned closer actually closes it. Each
// closer is idempotent; calling it more than once is a no-op.
func (s *Store) Open() func() {
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
func (s *Store) release() {
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
func (s *Store) openLocked() {
	options := badger.DefaultOptions(s.storagePath)
	options.Logger = nil
	options.MemTableSize = badgerMemTableSize
	options.NumMemtables = badgerNumMemtables
	options.BlockCacheSize = badgerBlockCacheSize

	if !is64Bit {
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

// Put stores value under key. The database must be open.
func (s *Store) Put(key, value []byte) error {
	s.checkCount()
	s.m.RLock()
	defer s.m.RUnlock()
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Get returns the value stored under key. The database must be open.
func (s *Store) Get(key []byte) ([]byte, error) {
	s.checkCount()
	s.m.RLock()
	defer s.m.RUnlock()

	var value []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return fmt.Errorf("reading stored value for path %s: %w", key, err)
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	return value, err
}

// checkCount reopens the database every reopenEvery operations.
func (s *Store) checkCount() {
	if s.counter.Add(1) < reopenEvery {
		return
	}
	s.m.Lock()
	defer s.m.Unlock()
	if s.counter.Load() < reopenEvery {
		// another goroutine already recycled the database
		return
	}
	s.counter.Store(0)
	if s.db != nil {
		s.db.Close()
	}
	s.openLocked()
}
