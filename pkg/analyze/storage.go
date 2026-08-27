package analyze

import (
	"bytes"
	"encoding/gob"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/dgraph-io/badger/v4"
	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/pkg/errors"
)

func init() {
	gob.RegisterName("analyze.StoredDir", &StoredDir{})
	gob.RegisterName("analyze.Dir", &Dir{})
	gob.RegisterName("analyze.File", &File{})
	gob.RegisterName("analyze.ParentDir", &ParentDir{})
}

// DefaultStorage is a default instance of badger storage
var DefaultStorage *Storage

// Storage represents a badger storage
//
// All access to db is serialized by m. Open is reference counted: badger takes
// an exclusive lock on the storage directory, so a second concurrent
// badger.Open on the same path would panic, and an inner caller's closer must
// not tear the database out from under an outer caller that is still using it.
type Storage struct {
	db          *badger.DB
	storagePath string
	topDir      string
	m           sync.RWMutex
	counter     atomic.Int64
	openCount   int
}

// NewStorage returns new instance of badger storage
func NewStorage(storagePath, topDir string) *Storage {
	st := &Storage{
		storagePath: storagePath,
		topDir:      topDir,
	}
	DefaultStorage = st
	return st
}

// GetTopDir returns top directory
func (s *Storage) GetTopDir() string {
	return s.topDir
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

// openLocked opens the badger DB. The caller must hold m for writing.
func (s *Storage) openLocked() {
	options := badger.DefaultOptions(s.storagePath)
	options.Logger = nil

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

// StoreDir saves item info into badger DB
func (s *Storage) StoreDir(dir fs.Item) error {
	s.checkCount()
	s.m.RLock()
	defer s.m.RUnlock()

	return s.db.Update(func(txn *badger.Txn) error {
		b := &bytes.Buffer{}
		enc := gob.NewEncoder(b)
		err := enc.Encode(dir)
		if err != nil {
			return errors.Wrap(err, "encoding dir value")
		}

		return txn.Set([]byte(dir.GetPath()), b.Bytes())
	})
}

// LoadDir saves item info into badger DB
func (s *Storage) LoadDir(dir fs.Item) error {
	s.checkCount()
	s.m.RLock()
	defer s.m.RUnlock()

	return s.db.View(func(txn *badger.Txn) error {
		path := dir.GetPath()
		item, err := txn.Get([]byte(path))
		if err != nil {
			return errors.Wrap(err, "reading stored value for path: "+path)
		}
		return item.Value(func(val []byte) error {
			b := bytes.NewBuffer(val)
			dec := gob.NewDecoder(b)
			return dec.Decode(dir)
		})
	})
}

// GetDirForPath returns Dir for given path
func (s *Storage) GetDirForPath(path string) (item fs.Item, err error) {
	dirPath := filepath.Dir(path)
	name := filepath.Base(path)
	dir := &StoredDir{
		Dir: &Dir{
			File: &File{
				Name: name,
			},
			BasePath: dirPath,
		},
	}
	err = s.LoadDir(dir)
	if err != nil {
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
