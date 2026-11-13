package app

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// isBadgerPath reports whether --db names a badger database rather than an
// SQLite one.
func isBadgerPath(dbPath string) bool {
	return strings.HasSuffix(dbPath, ".badger")
}

// checkDatabase refuses a --db path holding anything but a database gdu
// created. Scanning into it replaces it, and opening an SQLite file adds
// gdu's tables to it, so another file must stay untouched.
func checkDatabase(dbPath string) error {
	info, err := os.Lstat(dbPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking the database: %w", err)
	}
	if isBadgerPath(dbPath) {
		err = checkBadgerDatabase(dbPath, info)
	} else {
		err = checkSqliteDatabase(dbPath, info)
	}
	if err != nil {
		return fmt.Errorf("refusing to use %s as the database: %w", dbPath, err)
	}
	return nil
}

// badgerFile matches the names of the files badger keeps in its directory.
var badgerFile = regexp.MustCompile(`^(\d+\.(sst|vlog|mem)|(MANIFEST|KEYREGISTRY)(-REWRITE)?|DISCARD|LOCK)$`)

func checkBadgerDatabase(dbPath string, info os.FileInfo) error {
	if !info.IsDir() {
		return errors.New("it is not a directory")
	}
	entries, err := os.ReadDir(dbPath)
	if err != nil {
		return err
	}
	manifest := false
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !badgerFile.MatchString(entry.Name()) {
			return fmt.Errorf("%s is not a file of a badger database", entry.Name())
		}
		manifest = manifest || entry.Name() == "MANIFEST"
	}
	if len(entries) > 0 && !manifest {
		return errors.New("it is not a badger database")
	}
	return nil
}

// sqliteHeader starts every SQLite database file.
var sqliteHeader = []byte("SQLite format 3\x00")

// gduTables are the tables of gdu's SQLite database.
var gduTables = map[string]bool{"items": true, "metadata": true}

func checkSqliteDatabase(dbPath string, info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return errors.New("it is not a regular file")
	}
	if info.Size() == 0 {
		return nil
	}
	if err := checkSqliteHeader(dbPath); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", readOnlyURI(dbPath))
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if !gduTables[name] && !strings.HasPrefix(name, "sqlite_") {
			return fmt.Errorf("it holds the table %q, which gdu does not create", name)
		}
	}
	return rows.Err()
}

func checkSqliteHeader(dbPath string) error {
	file, err := os.Open(dbPath)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(file, header); err != nil || !bytes.Equal(header, sqliteHeader) {
		return errors.New("it is not an SQLite database")
	}
	return nil
}

// readOnlyURI returns the URI opening the SQLite database at dbPath read-only.
func readOnlyURI(dbPath string) string {
	if abs, err := filepath.Abs(dbPath); err == nil {
		dbPath = abs
	}
	slashed := filepath.ToSlash(dbPath)
	if !strings.HasPrefix(slashed, "/") {
		// a Windows path, C:/...
		slashed = "/" + slashed
	}
	return (&url.URL{Scheme: "file", Path: slashed, RawQuery: "mode=ro"}).String()
}

// replacesDatabase reports whether this run scans into --db, replacing what
// it held. A run reading a report or the database itself, or scanning
// sequentially, which stores nothing, leaves it as it is.
func (a *App) replacesDatabase() bool {
	return !a.Flags.ReadFromStorage && a.Flags.InputFile == "" && !a.Flags.SequentialScanning
}

// removeDatabase removes the database at --db, checked by checkDatabase.
func removeDatabase(dbPath string) error {
	var err error
	if isBadgerPath(dbPath) {
		err = os.RemoveAll(dbPath)
	} else {
		err = os.Remove(dbPath)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing the previous database: %w", err)
	}
	return nil
}
