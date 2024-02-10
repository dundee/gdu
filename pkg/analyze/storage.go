package analyze

import (
	"bytes"
	"encoding/gob"
	"path/filepath"

	"github.com/dgraph-io/badger/v3"
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
type Storage struct {
	db          *badger.DB
	storagePath string
	topDir      string
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
	return s.db != nil
}

// Open opens badger DB
func (s *Storage) Open() func() {
	options := badger.DefaultOptions(s.storagePath)
	options.Logger = nil
	db, err := badger.Open(options)
	if err != nil {
		panic(err)
	}
	s.db = db

	return func() {
		db.Close()
		s.db = nil
	}
}

// StoreDir saves item info into badger DB
func (s *Storage) StoreDir(dir fs.Item) error {
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
func (s *Storage) GetDirForPath(path string) (fs.Item, error) {
	dirPath := filepath.Dir(path)
	name := filepath.Base(path)
	dir := &StoredDir{
		&Dir{
			File: &File{
				Name: name,
			},
			BasePath: dirPath,
		},
		nil,
	}
	err := s.LoadDir(dir)
	if err != nil {
		return nil, err
	}
	return dir, nil
}
