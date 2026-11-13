package analyze

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// SqliteStorage represents SQLite database storage
type SqliteStorage struct {
	db         *sql.DB
	dbPath     string
	m          sync.RWMutex
	tx         *sql.Tx
	insertStmt *sql.Stmt
	updateStmt *sql.Stmt
}

// NewSqliteStorage creates a new SQLite storage and initializes the schema
func NewSqliteStorage(dbPath string) (*SqliteStorage, error) {
	parentDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create parent directory for SQLite database")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	storage := &SqliteStorage{
		db:     db,
		dbPath: dbPath,
	}

	if err := storage.createTables(); err != nil {
		db.Close()
		return nil, err
	}

	return storage, nil
}

// createTables creates the database schema if it doesn't exist
func (s *SqliteStorage) createTables() error {
	// Optimize for insertion speed
	pragmas := `
	PRAGMA synchronous = OFF;
	PRAGMA journal_mode = MEMORY;
	PRAGMA cache_size = -64000;
	PRAGMA temp_store = MEMORY;
	`
	if _, err := s.db.Exec(pragmas); err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS items (
		id          INTEGER PRIMARY KEY,
		parent_id   INTEGER REFERENCES items(id),
		name        TEXT NOT NULL,
		is_dir      INTEGER NOT NULL,
		size        INTEGER NOT NULL,
		usage       INTEGER NOT NULL,
		mtime       INTEGER NOT NULL,
		item_count  INTEGER NOT NULL DEFAULT 1,
		mli         INTEGER NOT NULL DEFAULT 0,
		flag        TEXT NOT NULL DEFAULT ' '
	);

	CREATE INDEX IF NOT EXISTS idx_items_parent_id ON items(parent_id);
	CREATE INDEX IF NOT EXISTS idx_items_parent_name ON items(parent_id, name);

	CREATE TABLE IF NOT EXISTS metadata (
		key   TEXT PRIMARY KEY,
		value TEXT
	);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *SqliteStorage) Close() error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// ClearItems removes all items from the database
func (s *SqliteStorage) ClearItems() error {
	_, err := s.db.Exec("DELETE FROM items")
	return err
}

// BeginBulkInsert starts a transaction and prepares statements for bulk insertion
func (s *SqliteStorage) BeginBulkInsert() error {
	s.m.Lock()
	defer s.m.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	s.tx = tx

	s.insertStmt, err = tx.Prepare(
		`INSERT INTO items (parent_id, name, is_dir, size, usage, mtime, item_count, mli, flag)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Errorf("failed to rollback transaction: %v", rollbackErr)
		}
		return err
	}

	s.updateStmt, err = tx.Prepare(
		`UPDATE items SET size = ?, usage = ?, item_count = ?, mtime = ?, flag = ? WHERE id = ?`,
	)
	if err != nil {
		s.insertStmt.Close()
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Errorf("failed to rollback transaction: %v", rollbackErr)
		}
		return err
	}

	return nil
}

// EndBulkInsert commits the transaction and closes prepared statements
func (s *SqliteStorage) EndBulkInsert() error {
	s.m.Lock()
	defer s.m.Unlock()

	if s.insertStmt != nil {
		s.insertStmt.Close()
		s.insertStmt = nil
	}
	if s.updateStmt != nil {
		s.updateStmt.Close()
		s.updateStmt = nil
	}
	if s.tx != nil {
		err := s.tx.Commit()
		s.tx = nil
		return err
	}
	return nil
}

// HasData returns true if the database contains analysis data
func (s *SqliteStorage) HasData() bool {
	s.m.RLock()
	defer s.m.RUnlock()

	var rowid int
	err := s.db.QueryRow("SELECT MAX(rowid) FROM items").Scan(&rowid)
	if err != nil {
		return false
	}
	return rowid > 0
}

// GetRootItem returns the root item (item with no parent)
func (s *SqliteStorage) GetRootItem() (*SqliteItem, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	item := &SqliteItem{storage: s}
	var parentID sql.NullInt64
	var isDirInt int
	var mtimeUnix int64
	var flag string

	err := s.db.QueryRow(
		`SELECT id, parent_id, name, is_dir, size, usage, mtime, item_count, mli, flag
		 FROM items WHERE parent_id IS NULL LIMIT 1`,
	).Scan(
		&item.id, &parentID, &item.name, &isDirInt,
		&item.size, &item.usage, &mtimeUnix, &item.itemCount,
		&item.mli, &flag,
	)
	if err != nil {
		return nil, err
	}

	item.isDir = isDirInt == 1
	item.mtime = time.Unix(mtimeUnix, 0)
	if flag != "" {
		item.flag = rune(flag[0])
	} else {
		item.flag = ' '
	}

	return item, nil
}

// InsertItem inserts a file/directory item into the database
func (s *SqliteStorage) InsertItem(
	parentID *int64, name string, isDir bool, size, usage int64, mtime time.Time, itemCount int64, mli uint64, flag rune,
) (int64, error) {
	isDirInt := 0
	if isDir {
		isDirInt = 1
	}

	var result sql.Result
	var err error

	// Use prepared statement if in bulk mode, otherwise use direct exec
	if s.insertStmt != nil {
		result, err = s.insertStmt.Exec(parentID, name, isDirInt, size, usage, mtime.Unix(), itemCount, mli, string(flag))
	} else {
		s.m.Lock()
		result, err = s.db.Exec(
			`INSERT INTO items (parent_id, name, is_dir, size, usage, mtime, item_count, mli, flag)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			parentID, name, isDirInt, size, usage, mtime.Unix(), itemCount, mli, string(flag),
		)
		s.m.Unlock()
	}
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// UpdateItem updates an existing item's stats
func (s *SqliteStorage) UpdateItem(id, size, usage, itemCount int64, mtime time.Time, flag rune) error {
	var err error

	// Use prepared statement if in bulk mode, otherwise use direct exec
	if s.updateStmt != nil {
		_, err = s.updateStmt.Exec(size, usage, itemCount, mtime.Unix(), string(flag), id)
	} else {
		s.m.Lock()
		_, err = s.db.Exec(
			`UPDATE items SET size = ?, usage = ?, item_count = ?, mtime = ?, flag = ? WHERE id = ?`,
			size, usage, itemCount, mtime.Unix(), string(flag), id,
		)
		s.m.Unlock()
	}
	return err
}

// deleteItemTree removes an item and all its descendants from the database within the given transaction.
func (s *SqliteStorage) deleteItemTree(tx *sql.Tx, id int64) error {
	query := `
	WITH RECURSIVE to_delete(id) AS (
		SELECT ?
		UNION ALL
		SELECT items.id FROM items JOIN to_delete ON items.parent_id = to_delete.id
	)
	DELETE FROM items WHERE id IN (SELECT id FROM to_delete)
	`
	_, err := tx.Exec(query, id)
	return err
}

// RemoveItemAndUpdateStats removes an item and updates all its ancestors' stats in the database
func (s *SqliteStorage) RemoveItemAndUpdateStats(id, size, usage, itemCount int64) error {
	s.m.Lock()
	defer s.m.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	// 1. Update all ancestors using recursive CTE
	updateAncestorsQuery := `
	WITH RECURSIVE ancestors(id, parent_id) AS (
		SELECT parent_id, NULL FROM items WHERE id = ?
		UNION ALL
		SELECT i.parent_id, NULL FROM items i JOIN ancestors a ON i.id = a.id WHERE i.parent_id IS NOT NULL
	)
	UPDATE items
	SET size = size - ?, usage = usage - ?, item_count = item_count - ?
	WHERE id IN (SELECT id FROM ancestors)
	`
	_, err = tx.Exec(updateAncestorsQuery, id, size, usage, itemCount)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Errorf("failed to rollback transaction: %v", rollbackErr)
		}
		return err
	}

	// 2. Delete the item and its descendants
	if err = s.deleteItemTree(tx, id); err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Errorf("failed to rollback transaction: %v", rollbackErr)
		}
		return err
	}

	return tx.Commit()
}

// GetChildByName returns a child item by its name and parent ID
func (s *SqliteStorage) GetChildByName(parentID int64, name string) (*SqliteItem, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	item := &SqliteItem{storage: s}
	var pID sql.NullInt64
	var isDirInt int
	var mtimeUnix int64
	var flag string

	err := s.db.QueryRow(
		`SELECT id, parent_id, name, is_dir, size, usage, mtime, item_count, mli, flag
		 FROM items WHERE parent_id = ? AND name = ? LIMIT 1`,
		parentID, name,
	).Scan(
		&item.id, &pID, &item.name, &isDirInt,
		&item.size, &item.usage, &mtimeUnix, &item.itemCount,
		&item.mli, &flag,
	)
	if err != nil {
		return nil, err
	}

	if pID.Valid {
		item.parentID = &pID.Int64
	}
	item.isDir = isDirInt == 1
	item.mtime = time.Unix(mtimeUnix, 0)
	if flag != "" {
		item.flag = rune(flag[0])
	} else {
		item.flag = ' '
	}

	return item, nil
}

// GetChildren returns all children of a given parent ID
func (s *SqliteStorage) GetChildren(parentID int64) ([]*SqliteItem, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, parent_id, name, is_dir, size, usage, mtime, item_count, mli, flag
		 FROM items WHERE parent_id = ?`,
		parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*SqliteItem
	for rows.Next() {
		item := &SqliteItem{storage: s}
		var parentID sql.NullInt64
		var isDirInt int
		var mtimeUnix int64
		var flag string

		err := rows.Scan(
			&item.id, &parentID, &item.name, &isDirInt,
			&item.size, &item.usage, &mtimeUnix, &item.itemCount,
			&item.mli, &flag,
		)
		if err != nil {
			return nil, err
		}

		if parentID.Valid {
			item.parentID = &parentID.Int64
		}
		item.isDir = isDirInt == 1
		item.mtime = time.Unix(mtimeUnix, 0)
		if flag != "" {
			item.flag = rune(flag[0])
		} else {
			item.flag = ' '
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// GetItemByID returns an item by its ID
func (s *SqliteStorage) GetItemByID(id int64) (*SqliteItem, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	item := &SqliteItem{storage: s}
	var parentID sql.NullInt64
	var isDirInt int
	var mtimeUnix int64
	var flag string

	err := s.db.QueryRow(
		`SELECT id, parent_id, name, is_dir, size, usage, mtime, item_count, mli, flag
		 FROM items WHERE id = ?`,
		id,
	).Scan(
		&item.id, &parentID, &item.name, &isDirInt,
		&item.size, &item.usage, &mtimeUnix, &item.itemCount,
		&item.mli, &flag,
	)
	if err != nil {
		return nil, err
	}

	if parentID.Valid {
		item.parentID = &parentID.Int64
	}
	item.isDir = isDirInt == 1
	item.mtime = time.Unix(mtimeUnix, 0)
	if flag != "" {
		item.flag = rune(flag[0])
	} else {
		item.flag = ' '
	}

	return item, nil
}

// SetMetadata stores a metadata key-value pair
func (s *SqliteStorage) SetMetadata(key, value string) error {
	s.m.Lock()
	defer s.m.Unlock()

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO metadata (key, value) VALUES (?, ?)`,
		key, value,
	)
	return err
}

// GetMetadata retrieves a metadata value by key
func (s *SqliteStorage) GetMetadata(key string) (string, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	var value string
	err := s.db.QueryRow(`SELECT value FROM metadata WHERE key = ?`, key).Scan(&value)
	return value, err
}

// SqliteItem represents a file or directory stored in SQLite
type SqliteItem struct {
	storage   *SqliteStorage
	id        int64
	parentID  *int64
	name      string
	isDir     bool
	size      int64
	usage     int64
	mtime     time.Time
	itemCount int64
	mli       uint64
	flag      rune
	parent    fs.Item
	m         sync.RWMutex
}

// GetPath returns the full path of the item
func (i *SqliteItem) GetPath() string {
	parent := i.GetParent()
	if parent != nil {
		return filepath.Join(parent.GetPath(), i.name)
	}
	// For root item, get basePath from metadata
	basePath, err := i.storage.GetMetadata("top_dir_path")
	if err != nil {
		return i.name
	}
	return filepath.Join(filepath.Dir(basePath), i.name)
}

// GetName returns the name of the item
func (i *SqliteItem) GetName() string {
	return i.name
}

// GetFlag returns the flag of the item
func (i *SqliteItem) GetFlag() rune {
	return i.flag
}

// IsDir returns true if the item is a directory
func (i *SqliteItem) IsDir() bool {
	return i.isDir
}

// GetSize returns the apparent size
func (i *SqliteItem) GetSize() int64 {
	i.m.RLock()
	defer i.m.RUnlock()
	return i.size
}

// GetType returns the type of the item
func (i *SqliteItem) GetType() string {
	if i.isDir {
		return "Directory"
	}
	if i.flag == '@' {
		return "Other"
	}
	return "File"
}

// GetUsage returns the disk usage
func (i *SqliteItem) GetUsage() int64 {
	i.m.RLock()
	defer i.m.RUnlock()
	return i.usage
}

// GetMtime returns the modification time
func (i *SqliteItem) GetMtime() time.Time {
	i.m.RLock()
	defer i.m.RUnlock()
	return i.mtime
}

// GetItemCount returns the item count
func (i *SqliteItem) GetItemCount() int64 {
	i.m.RLock()
	defer i.m.RUnlock()
	return i.itemCount
}

// GetParent returns the parent item
func (i *SqliteItem) GetParent() fs.Item {
	i.m.RLock()
	if i.parent != nil {
		defer i.m.RUnlock()
		return i.parent
	}
	if i.parentID == nil {
		defer i.m.RUnlock()
		return nil
	}
	i.m.RUnlock()

	parent, err := i.storage.GetItemByID(*i.parentID)
	if err != nil {
		log.Print(err.Error())
		return nil
	}

	i.m.Lock()
	defer i.m.Unlock()
	if i.parent == nil {
		i.parent = parent
	}
	return i.parent
}

// SetParent sets the parent item
func (i *SqliteItem) SetParent(parent fs.Item) {
	i.parent = parent
}

// GetParentLocked returns the in-memory parent without hitting the database.
// Used inside RemoveFile where storage.m is already write-locked.
func (i *SqliteItem) GetParentLocked() *SqliteItem {
	if i.parent != nil {
		return i.parent.(*SqliteItem)
	}
	return nil
}

// GetMultiLinkedInode returns the multi-linked inode number
func (i *SqliteItem) GetMultiLinkedInode() uint64 {
	return i.mli
}

// EncodeJSON encodes the item to JSON
func (i *SqliteItem) EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
	if i.isDir {
		return i.encodeDirJSON(writer, topLevel, attributes)
	}
	return i.encodeFileJSON(writer, attributes)
}

func (i *SqliteItem) encodeDirJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
	buff := make([]byte, 0, 128)
	buff = append(buff, []byte(`[{"name":`)...)

	name := i.GetName()
	if topLevel {
		name = i.GetPath()
	}
	if err := addSqliteString(&buff, name); err != nil {
		return err
	}

	// Directory summary stats (asize, dsize, items) are written by default so
	// they can be preserved on import. When --output-attrs is supplied, each is
	// emitted only if explicitly selected.
	if attributes.Includes("asize") {
		buff = append(buff, []byte(`,"asize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(i.GetSize(), 10))...)
	}
	if attributes.Includes("dsize") {
		buff = append(buff, []byte(`,"dsize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(i.GetUsage(), 10))...)
	}
	if attributes.Includes("items") {
		buff = append(buff, []byte(`,"items":`)...)
		buff = append(buff, []byte(strconv.FormatInt(i.GetItemCount(), 10))...)
	}
	if attributes.Includes("mtime") && !i.GetMtime().IsZero() {
		buff = append(buff, []byte(`,"mtime":`)...)
		buff = append(buff, []byte(strconv.FormatInt(i.GetMtime().Unix(), 10))...)
	}

	buff = append(buff, '}')

	children, err := i.storage.GetChildren(i.id)
	if err != nil {
		return err
	}

	if len(children) > 0 {
		buff = append(buff, ',')
	}
	buff = append(buff, '\n')

	if _, err := writer.Write(buff); err != nil {
		return err
	}

	for idx, child := range children {
		if idx > 0 {
			if _, err := writer.Write([]byte(",\n")); err != nil {
				return err
			}
		}
		child.parent = i
		if err := child.EncodeJSON(writer, false, attributes); err != nil {
			return err
		}
	}

	if _, err := writer.Write([]byte("]")); err != nil {
		return err
	}
	return nil
}

func (i *SqliteItem) encodeFileJSON(writer io.Writer, attributes fs.JSONAttributes) error {
	buff := make([]byte, 0, 128)
	buff = append(buff, []byte(`{"name":`)...)
	if err := addSqliteString(&buff, i.GetName()); err != nil {
		return err
	}
	if attributes.Includes("asize") && i.GetSize() > 0 {
		buff = append(buff, []byte(`,"asize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(i.GetSize(), 10))...)
	}
	if attributes.Includes("dsize") && i.GetUsage() > 0 {
		buff = append(buff, []byte(`,"dsize":`)...)
		buff = append(buff, []byte(strconv.FormatInt(i.GetUsage(), 10))...)
	}
	if attributes.Includes("mtime") && !i.GetMtime().IsZero() {
		buff = append(buff, []byte(`,"mtime":`)...)
		buff = append(buff, []byte(strconv.FormatInt(i.GetMtime().Unix(), 10))...)
	}

	if attributes.Includes("notreg") && i.flag == '@' {
		buff = append(buff, []byte(`,"notreg":true`)...)
	}
	if attributes.Includes("ino") && i.flag == 'H' {
		buff = append(buff, []byte(`,"ino":`+strconv.FormatUint(i.mli, 10)+`,"hlnkc":true`)...)
	}

	buff = append(buff, '}')

	if _, err := writer.Write(buff); err != nil {
		return err
	}
	return nil
}

func addSqliteString(buff *[]byte, val string) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	*buff = append(*buff, b...)
	return nil
}

// GetItemStats returns item statistics - hard links already handled during scan
func (i *SqliteItem) GetItemStats(*fs.HardLinks, bool) (itemCount, size, usage int64) {
	return i.itemCount, i.size, i.usage
}

// UpdateStats does nothing for SqliteItem: the totals are computed, and hard
// links counted once, during the scan.
func (i *SqliteItem) UpdateStats(bool) *fs.HardLinks { return nil }

// AddFile adds a child file (no-op for SQLite items - children are in DB)
func (i *SqliteItem) AddFile(item fs.Item) {
	// Children are stored in database via parent_id relationship
}

// GetFiles returns children as a sorted iterator
func (i *SqliteItem) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {
		children, err := i.storage.GetChildren(i.id)
		if err != nil {
			log.Print(err.Error())
			return
		}

		// Convert to fs.Files for sorting
		files := make(fs.Files, len(children))
		for idx, child := range children {
			child.parent = i
			files[idx] = child
		}

		sortFiles(files, sortBy, order)

		for _, item := range files {
			if !yield(item) {
				return
			}
		}
	}
}

// RemoveFile removes a child file, updating both in-memory stats and the database in one pass.
func (i *SqliteItem) RemoveFile(item fs.Item) {
	sqliteItem := item.(*SqliteItem)
	size := item.GetSize()
	usage := item.GetUsage()
	itemCount := item.GetItemCount()

	// Apply in-memory stats updates for loaded ancestors first,
	// to avoid double-counting if ancestors are loaded from DB after it's updated.
	cur := i
	for {
		cur.m.Lock()
		cur.size -= size
		cur.usage -= usage
		cur.itemCount -= itemCount
		cur.m.Unlock()

		parent := cur.GetParentLocked()
		if parent == nil {
			break
		}
		cur = parent
	}

	if err := i.storage.RemoveItemAndUpdateStats(sqliteItem.id, size, usage, itemCount); err != nil {
		log.Errorf("Error removing item and updating stats: %v", err)
		return
	}
}

// RemoveFileByName removes a child by name
func (i *SqliteItem) RemoveFileByName(name string) {
	child, err := i.storage.GetChildByName(i.id, name)
	if err != nil {
		log.Errorf("Error getting child from database: %v", err)
		return
	}

	i.RemoveFile(child)
}

// SqliteAnalyzer implements Analyzer using SQLite storage
type SqliteAnalyzer struct {
	BaseAnalyzer
	storage   *SqliteStorage
	dbWriteMu sync.Mutex
}

// insertItemLocked is a serialized wrapper around storage.InsertItem.
// All concurrent goroutines spawned during AnalyzeDir must use this helper
// because the underlying *sql.Tx and prepared statements are not safe for
// concurrent use.
func (a *SqliteAnalyzer) insertItemLocked(
	parentID *int64, name string, isDir bool, size, usage int64, mtime time.Time,
	itemCount int64, mli uint64, flag rune,
) (int64, error) {
	a.dbWriteMu.Lock()
	defer a.dbWriteMu.Unlock()
	return a.storage.InsertItem(parentID, name, isDir, size, usage, mtime, itemCount, mli, flag)
}

func (a *SqliteAnalyzer) updateDirLocked(id, size, usage, itemCount int64, mtime time.Time, flag rune) error {
	a.dbWriteMu.Lock()
	defer a.dbWriteMu.Unlock()
	return a.storage.UpdateItem(id, size, usage, itemCount, mtime, flag)
}

// CreateSqliteAnalyzer creates a new SQLite analyzer
func CreateSqliteAnalyzer(dbPath string) (*SqliteAnalyzer, error) {
	if err := checkAvailable(); err != nil {
		return nil, err
	}

	storage, err := NewSqliteStorage(dbPath)
	if err != nil {
		return nil, err
	}

	a := &SqliteAnalyzer{
		storage: storage,
	}
	a.Init()
	return a, nil
}

// LoadAnalysis returns the analysis of path stored in the database by an
// earlier scan. It fails when the database holds no analysis, or the analysis
// of another path.
func (a *SqliteAnalyzer) LoadAnalysis(path string) (fs.Item, error) {
	if !a.storage.HasData() {
		return nil, fmt.Errorf("the database holds no analysis of %s", path)
	}
	stored, err := a.storage.GetMetadata("top_dir_path")
	if err != nil {
		return nil, fmt.Errorf("reading the path of the stored analysis: %w", err)
	}
	if filepath.Clean(stored) != filepath.Clean(path) {
		return nil, fmt.Errorf("the database holds the analysis of %s, not of %s", stored, path)
	}
	return a.storage.GetRootItem()
}

// AnalyzeDir analyzes the given path and stores results in SQLite, replacing
// whatever the database held before. LoadAnalysis reads a stored analysis.
func (a *SqliteAnalyzer) AnalyzeDir(ctx context.Context, path string, opts common.ScanOptions) fs.Item {
	s := a.begin(ctx, opts)

	// Clear existing data and store metadata
	err := a.storage.ClearItems()
	if err != nil {
		log.Printf("Error clearing items: %v", err)
	}
	err = a.storage.SetMetadata("top_dir_path", path)
	if err != nil {
		log.Printf("Error setting metadata: %v", err)
	}

	// Start bulk insert transaction
	if err := a.storage.BeginBulkInsert(); err != nil {
		log.Printf("Error starting bulk insert: %v", err)
	}

	root := walkTree[*sqliteDir](&a.BaseAnalyzer, s, sqliteSink{a: a, scan: s}, path, true)

	// Commit bulk insert transaction
	if err := a.storage.EndBulkInsert(); err != nil {
		log.Printf("Error committing bulk insert: %v", err)
	}

	if root == nil {
		return nil
	}
	return root.item
}

// sqliteDir is a directory of a scan into SQLite.
type sqliteDir struct {
	id       int64
	parentID *int64
	name     string
	totals   *runningTotals
	// item is the directory as stored, once it is closed
	item *SqliteItem
}

// sqliteSink inserts every directory and file of a scan into SQLite, totaling
// the directories as it goes: a directory's row is completed once everything
// below it has been inserted.
type sqliteSink struct {
	a    *SqliteAnalyzer
	scan *scan
}

func (s sqliteSink) openDir(parent *sqliteDir, path string, root bool) (*sqliteDir, bool) {
	dir := &sqliteDir{name: filepath.Base(path)}
	if !root {
		dir.parentID = &parent.id
	}
	mtime, ownUsage, _ := dirStats(path)
	dir.totals = newRunningTotals(mtime, ownUsage)

	// the totals are stored when the directory is closed
	id, err := s.a.insertItemLocked(dir.parentID, dir.name, true, 0, 0, mtime, 1, 0, ' ')
	if err != nil {
		log.Print(err.Error())
		return nil, false
	}
	dir.id = id
	return dir, true
}

func (sqliteSink) readDir(dir *sqliteDir, _ string, err error, entries int) {
	dir.totals.setFlag(getDirFlag(err, entries))
}

func (s sqliteSink) resolveOptions(*sqliteDir) resolveOptions {
	return resolveOptions{browseArchives: true, links: &s.scan.links}
}

func (s sqliteSink) addFile(dir *sqliteDir, entry *fileEntry) int64 {
	mtime := entry.info.ModTime()
	if archive := entry.archive; archive != nil {
		archiveID, err := s.a.insertItemLocked(
			&dir.id, entry.name, true,
			archive.GetSize(), archive.GetUsage(), mtime, archive.GetItemCount(), 0, archive.GetFlag(),
		)
		if err != nil {
			log.Print(err.Error())
			return 0
		}
		s.a.persistArchive(s.scan, archive, archiveID)
		dir.totals.addEntry(archive.GetItemCount(), archive.GetSize(), archive.GetUsage(), mtime, archive.GetFlag(), true)
		return archive.GetUsage()
	}

	// a file met through another hard link before is counted there
	size, usage := entry.info.Size(), entry.usage
	if entry.duplicate {
		size, usage = 0, 0
	}
	_, err := s.a.insertItemLocked(&dir.id, entry.name, false, size, usage, mtime, 1, entry.link.ino, entry.flag)
	if err != nil {
		log.Print(err.Error())
		return 0
	}
	dir.totals.addEntry(1, size, usage, mtime, entry.flag, false)
	return usage
}

func (sqliteSink) fileError(dir *sqliteDir) {
	dir.totals.setFlag('!')
}

func (sqliteSink) entriesDone(*sqliteDir) {}

func (s sqliteSink) closeDir(dir *sqliteDir) {
	itemCount, size, usage, mtime, flag := dir.totals.result(s.scan.filteringFiles())
	if err := s.a.updateDirLocked(dir.id, size, usage, itemCount, mtime, flag); err != nil {
		log.Printf("Error updating item: %v", err)
	}
	dir.item = &SqliteItem{
		storage:   s.a.storage,
		id:        dir.id,
		parentID:  dir.parentID,
		name:      dir.name,
		isDir:     true,
		size:      size,
		usage:     usage,
		mtime:     mtime,
		itemCount: itemCount,
		flag:      flag,
	}
}

func (sqliteSink) addDir(parent, child *sqliteDir) {
	item := child.item
	parent.totals.addEntry(item.itemCount, item.size, item.usage, item.mtime, item.flag, true)
}

// persistArchive stores the entries of a browsed archive, or of a directory
// in one, under parentID.
func (a *SqliteAnalyzer) persistArchive(s *scan, archive fs.Item, parentID int64) {
	archiveDir := archiveDirOf(archive)
	if archiveDir == nil {
		return
	}
	for _, f := range archiveDir.Files {
		if s.isCancelled() {
			return
		}
		if f.IsDir() {
			id, err := a.insertItemLocked(
				&parentID,
				f.GetName(),
				true,
				f.GetSize(),
				f.GetUsage(),
				f.GetMtime(),
				f.GetItemCount(),
				0,
				f.GetFlag(),
			)
			if err != nil {
				log.Print(err.Error())
				continue
			}
			a.persistArchive(s, f, id)
		} else {
			_, err := a.insertItemLocked(
				&parentID,
				f.GetName(),
				false,
				f.GetSize(),
				f.GetUsage(),
				f.GetMtime(),
				1,
				f.GetMultiLinkedInode(),
				f.GetFlag(),
			)
			if err != nil {
				log.Print(err.Error())
				continue
			}
		}
	}
}
