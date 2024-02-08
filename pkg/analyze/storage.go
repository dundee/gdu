package analyze

import (
	"bytes"
	"encoding/gob"

	"github.com/dgraph-io/badger/v3"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/pkg/errors"
)

func init() {
	gob.RegisterName("analyze.StoredDir", &StoredDir{})
	gob.RegisterName("analyze.Dir", &Dir{})
	gob.RegisterName("analyze.File", &File{})
}

var DefaultStorage *Storage

type Storage struct {
	db     *badger.DB
	topDir string
}

func NewStorage(topDir string) *Storage {
	st := &Storage{
		topDir: topDir,
	}
	DefaultStorage = st
	return st
}

// GetTopDir returns top directory
func (s *Storage) GetTopDir() string {
	return s.topDir
}

func (s *Storage) IsOpen() bool {
	return s.db != nil
}

func (s *Storage) Open() func() {
	options := badger.DefaultOptions("/tmp/badger")
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

		txn.Set([]byte(dir.GetPath()), b.Bytes())
		return nil
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
