// Package sqlite creates analyzers storing their results in an SQLite
// database, the way gdu does with --db x.sqlite, using the pure Go driver
// modernc.org/sqlite.
//
// It is a package of its own so that programs using package analyze without
// it do not link SQLite.
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

// CreateAnalyzer returns an analyzer storing its results in the SQLite
// database at path. The database, and the directories leading to it, are
// created when missing.
//
// It fails on the platforms modernc.org/sqlite does not support.
func CreateAnalyzer(path string) (*analyze.SqliteAnalyzer, error) {
	if err := checkAvailable(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create parent directory for SQLite database: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return analyze.NewSqliteAnalyzer(db)
}
