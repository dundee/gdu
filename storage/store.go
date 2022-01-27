package storage

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// StoreDir saves item info into badger DB
func StoreDir(dir fs.Item) {
	options := badger.DefaultOptions("/tmp/badger")
	options.Logger = nil
	db, err := badger.Open(options)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.Update(func(txn *badger.Txn) error {
		txn.Set([]byte(dir.GetPath()), []byte(dir.GetName()))
		return nil
	})
}
