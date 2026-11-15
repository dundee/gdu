package analyze

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	"github.com/dundee/gdu/v6/internal/badgerstore"
)

// The constructors of packages analyze/badger and analyze/sqlite, which
// import this package and so cannot be used by its tests.

// storedAnalyzers are the analyzers createStoredAnalyzer made, by path.
var storedAnalyzers sync.Map

// createStoredAnalyzer returns an analyzer of the badger database at path,
// as a later run of gdu would open it: the analyzer made for path before is
// closed, as only one can open a badger database at a time.
func createStoredAnalyzer(path string) *StoredAnalyzer {
	if previous, ok := storedAnalyzers.Load(path); ok {
		_ = previous.(*StoredAnalyzer).Close()
	}
	a := NewStoredAnalyzer(badgerstore.New(path))
	storedAnalyzers.Store(path, a)
	return a
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
