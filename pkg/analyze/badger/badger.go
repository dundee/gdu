// Package badger creates analyzers storing their results in a badger
// database, the way gdu does with --db x.badger. Only the directories in use
// are kept in memory, the rest are loaded from the database when needed.
//
// It is a package of its own so that programs using package analyze without
// it do not link badger.
package badger

import (
	"github.com/dundee/gdu/v5/internal/badgerstore"
	"github.com/dundee/gdu/v5/pkg/analyze"
)

// CreateAnalyzer returns an analyzer storing its results in the badger
// database in the directory path, which is created when missing. The
// database is opened while the analyzer or the tree it returns uses it.
func CreateAnalyzer(path string) *analyze.StoredAnalyzer {
	return analyze.NewStoredAnalyzer(badgerstore.New(path))
}
