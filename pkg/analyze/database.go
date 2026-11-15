package analyze

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
)

// Database is an analyzer keeping each scan in a database (--db), so that a
// later run can load the analysis instead of scanning again. Packages
// analyze/badger and analyze/sqlite create one.
//
// AnalyzeDir replaces what the database held with the scan of path. The
// analysis is stored once the totals of the scan are: SQLite stores them as
// it scans, badger when UpdateStats computes them on the returned tree. It is
// then complete, or stopped when the scan was cancelled: what was found so
// far is kept, as for any stopped scan. A scan which never got there, as when
// gdu was killed, or which failed to store part of the analysis, leaves an
// incomplete one.
//
// LoadAnalysis returns the stored analysis of path, complete or stopped. It
// fails with ErrNoAnalysis when the database holds no analysis of path, with
// ErrIncompleteAnalysis when it is incomplete, and with ErrStoredFormat when
// another version of gdu wrote it.
//
// The database stays open until Close, which a program calls once it is done
// with the analyzer and the trees it returned. A database opened by one
// analyzer cannot be opened by another until then.
type Database interface {
	Analyzer
	LoadAnalysis(path string) (Node, error)
	Close() error
}

var (
	_ Database = (*StoredAnalyzer)(nil)
	_ Database = (*SqliteAnalyzer)(nil)
)

var (
	// ErrNoAnalysis is returned when a database holds no analysis of the
	// path asked for.
	ErrNoAnalysis = errors.New("the database holds no analysis")
	// ErrIncompleteAnalysis is returned when the scan which stored the
	// analysis never finished, or failed to store part of it.
	ErrIncompleteAnalysis = errors.New("the stored analysis is incomplete, scan again")
	// ErrStoredFormat is returned for a database written by another version
	// of gdu.
	ErrStoredFormat = errors.New("the database was written by another version of gdu, scan again to replace it")
)

// analysisState is how far the scan which stored an analysis got.
type analysisState string

const (
	// analysisUnfinished is the state while the scan runs, and stays when it
	// never finishes or fails to store part of the analysis
	analysisUnfinished analysisState = "unfinished"
	analysisStopped    analysisState = "stopped"
	analysisComplete   analysisState = "complete"
)

// analysisInfo is what a database records about the analysis it holds.
type analysisInfo struct {
	// Format is the format of the database, see each backend
	Format int           `json:"format"`
	Root   string        `json:"root"`
	State  analysisState `json:"state"`
}

// infoStore is a database recording what analysis it holds.
type infoStore interface {
	// writeInfo records info, replacing what was recorded
	writeInfo(info analysisInfo) error
	// readInfo returns what is recorded; ok is false when nothing is
	readInfo() (info analysisInfo, ok bool, err error)
	// holdsUnrecorded reports whether the database holds an analysis of
	// path by a gdu which recorded nothing about it, from before analysisInfo
	holdsUnrecorded(path string) (bool, error)
}

// beginAnalysis records in db that the scan of root into it has started,
// with the database format format.
func beginAnalysis(db infoStore, format int, root string) error {
	return db.writeInfo(analysisInfo{Format: format, Root: root, State: analysisUnfinished})
}

// endAnalysis records in db that the analysis of root is stored, stopped or
// complete. When writing part of it failed, the analysis stays unfinished.
func endAnalysis(db infoStore, format int, root string, stopped, failed bool) error {
	if failed {
		return fmt.Errorf("storing the analysis of %s failed, it is left incomplete", root)
	}
	state := analysisComplete
	if stopped {
		state = analysisStopped
	}
	return db.writeInfo(analysisInfo{Format: format, Root: root, State: state})
}

// checkAnalysis returns nil when db holds a loadable analysis of path in the
// database format format, and the error telling why not otherwise.
func checkAnalysis(db infoStore, format int, path string) error {
	info, ok, err := db.readInfo()
	switch {
	case errors.Is(err, ErrStoredFormat):
		return ErrStoredFormat
	case err != nil:
		return fmt.Errorf("reading the database: %w", err)
	case !ok:
		return checkUnrecorded(db, path)
	case info.Format != format:
		return ErrStoredFormat
	case filepath.Clean(info.Root) != filepath.Clean(path):
		return fmt.Errorf("%w of %s, it holds the analysis of %s", ErrNoAnalysis, path, info.Root)
	case info.State == analysisUnfinished:
		return fmt.Errorf("%s: %w", path, ErrIncompleteAnalysis)
	case info.State != analysisComplete && info.State != analysisStopped:
		return ErrStoredFormat
	}
	return nil
}

// checkUnrecorded tells why db, which records no analysis, holds no loadable
// analysis of path.
func checkUnrecorded(db infoStore, path string) error {
	old, err := db.holdsUnrecorded(path)
	switch {
	case err != nil:
		return fmt.Errorf("reading the database: %w", err)
	case old:
		return ErrStoredFormat
	}
	return fmt.Errorf("%w of %s", ErrNoAnalysis, path)
}

func (i analysisInfo) marshal() ([]byte, error) {
	return json.Marshal(i)
}

// unmarshalInfo decodes info, failing with ErrStoredFormat when data is no
// analysisInfo.
func unmarshalInfo(data []byte) (analysisInfo, error) {
	var info analysisInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("%w: %w", ErrStoredFormat, err)
	}
	return info, nil
}
