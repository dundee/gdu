package analyze

import "errors"

// Database is an analyzer keeping each scan in a database (--db), so that a
// later run can load the analysis instead of scanning again. Packages
// analyze/badger and analyze/sqlite create one.
//
// AnalyzeDir replaces what the database held with the scan of path.
// LoadAnalysis returns the stored analysis of path. It fails with
// ErrNoAnalysis when the database holds no analysis of path, and with
// ErrStoredFormat when another version of gdu wrote it.
type Database interface {
	Analyzer
	LoadAnalysis(path string) (Node, error)
}

var (
	_ Database = (*StoredAnalyzer)(nil)
	_ Database = (*SqliteAnalyzer)(nil)
)

var (
	// ErrNoAnalysis is returned when a database holds no analysis of the
	// path asked for.
	ErrNoAnalysis = errors.New("the database holds no analysis")
	// ErrStoredFormat is returned for a database written by another version
	// of gdu.
	ErrStoredFormat = errors.New("the database was written by another version of gdu, scan again to replace it")
)
