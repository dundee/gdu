package analyze

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/dundee/gdu/v5/internal/badgerstore"
)

// The constructors of packages analyze/badger and analyze/sqlite, which
// import this package and so cannot be used by its tests.

func createStoredAnalyzer(path string) *StoredAnalyzer {
	return NewStoredAnalyzer(badgerstore.New(path))
}

func createSqliteAnalyzer(path string) (*SqliteAnalyzer, error) {
	db, err := openSqlite(path)
	if err != nil {
		return nil, err
	}
	return NewSqliteAnalyzer(db)
}

func openSqliteStorage(path string) (*SqliteStorage, error) {
	db, err := openSqlite(path)
	if err != nil {
		return nil, err
	}
	return newSqliteStorage(db)
}

func openSqlite(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", path)
}

// storageIsOpen reports whether the badger database of a is open.
func storageIsOpen(a *StoredAnalyzer) bool {
	return a.storage.RecordStore.(*badgerstore.Store).IsOpen()
}
