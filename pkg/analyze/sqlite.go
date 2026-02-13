package analyze

import (
	"database/sql"
	"io"
	"iter"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

// SqliteStorage represents SQLite database storage
type SqliteStorage struct {
	db           *sql.DB
	dbPath       string
	m            sync.RWMutex
	tx           *sql.Tx
	insertStmt   *sql.Stmt
	updateStmt   *sql.Stmt
	hasInodeStmt *sql.Stmt
}

// NewSqliteStorage creates a new SQLite storage and initializes the schema
func NewSqliteStorage(dbPath string) (*SqliteStorage, error) {
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
	CREATE INDEX IF NOT EXISTS idx_items_mli ON items(mli) WHERE mli != 0;

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
		`UPDATE items SET size = ?, usage = ?, item_count = ? WHERE id = ?`,
	)
	if err != nil {
		s.insertStmt.Close()
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Errorf("failed to rollback transaction: %v", rollbackErr)
		}
		return err
	}

	s.hasInodeStmt, err = tx.Prepare(
		`SELECT 1 FROM items WHERE mli = ? LIMIT 1`,
	)
	if err != nil {
		s.insertStmt.Close()
		s.updateStmt.Close()
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
	if s.hasInodeStmt != nil {
		s.hasInodeStmt.Close()
		s.hasInodeStmt = nil
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

// HasInode returns true if a file with the given inode already exists in the database
func (s *SqliteStorage) HasInode(mli uint64) bool {
	var exists int
	var err error

	if s.hasInodeStmt != nil {
		err = s.hasInodeStmt.QueryRow(mli).Scan(&exists)
	} else {
		s.m.RLock()
		err = s.db.QueryRow(`SELECT 1 FROM items WHERE mli = ? LIMIT 1`, mli).Scan(&exists)
		s.m.RUnlock()
	}

	return err == nil
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
	parentID *int64, name string, isDir bool, size, usage int64, mtime time.Time, itemCount int, mli uint64, flag rune,
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
func (s *SqliteStorage) UpdateItem(id, size, usage int64, itemCount int) error {
	var err error

	// Use prepared statement if in bulk mode, otherwise use direct exec
	if s.updateStmt != nil {
		_, err = s.updateStmt.Exec(size, usage, itemCount, id)
	} else {
		s.m.Lock()
		_, err = s.db.Exec(
			`UPDATE items SET size = ?, usage = ?, item_count = ? WHERE id = ?`,
			size, usage, itemCount, id,
		)
		s.m.Unlock()
	}
	return err
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
	itemCount int
	mli       uint64
	flag      rune
	parent    fs.Item
	m         sync.RWMutex
}

// GetPath returns the full path of the item
func (i *SqliteItem) GetPath() string {
	if i.parent != nil {
		return filepath.Join(i.parent.GetPath(), i.name)
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
	return i.usage
}

// GetMtime returns the modification time
func (i *SqliteItem) GetMtime() time.Time {
	return i.mtime
}

// GetItemCount returns the item count
func (i *SqliteItem) GetItemCount() int {
	return i.itemCount
}

// GetParent returns the parent item
func (i *SqliteItem) GetParent() fs.Item {
	if i.parent != nil {
		return i.parent
	}
	if i.parentID == nil {
		return nil
	}

	parent, err := i.storage.GetItemByID(*i.parentID)
	if err != nil {
		log.Print(err.Error())
		return nil
	}
	i.parent = parent
	return parent
}

// SetParent sets the parent item
func (i *SqliteItem) SetParent(parent fs.Item) {
	i.parent = parent
}

// GetMultiLinkedInode returns the multi-linked inode number
func (i *SqliteItem) GetMultiLinkedInode() uint64 {
	return i.mli
}

// EncodeJSON encodes the item to JSON
func (i *SqliteItem) EncodeJSON(writer io.Writer, topLevel bool) error {
	// Delegate to standard encoding logic
	// This is a simplified version - full implementation would mirror Dir.EncodeJSON
	return nil
}

// GetItemStats returns item statistics - hard links already handled during scan
func (i *SqliteItem) GetItemStats(linkedItems fs.HardLinkedItems) (itemCount int, size, usage int64) {
	return i.itemCount, i.size, i.usage
}

// UpdateStats is a no-op for SqliteItem - hard links are handled during scan
func (i *SqliteItem) UpdateStats(linkedItems fs.HardLinkedItems) {
}

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

// GetFilesLocked returns children with locking
func (i *SqliteItem) GetFilesLocked(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return i.GetFiles(sortBy, order)
}

// RemoveFile removes a child file
func (i *SqliteItem) RemoveFile(item fs.Item) {
	// TODO: implement deletion from database
}

// RemoveFileByName removes a child by name
func (i *SqliteItem) RemoveFileByName(name string) {
	// TODO: implement deletion from database
}

// RLock returns a no-op unlock function
func (i *SqliteItem) RLock() func() {
	i.m.RLock()
	return i.m.RUnlock
}

// SqliteAnalyzer implements Analyzer using SQLite storage
type SqliteAnalyzer struct {
	storage             *SqliteStorage
	progress            *common.CurrentProgress
	progressChan        chan common.CurrentProgress
	progressOutChan     chan common.CurrentProgress
	progressDoneChan    chan struct{}
	doneChan            common.SignalGroup
	wait                *WaitGroup
	ignoreDir           common.ShouldDirBeIgnored
	ignoreFileType      common.ShouldFileBeIgnored
	followSymlinks      bool
	gitAnnexedSize      bool
	matchesTimeFilterFn common.TimeFilter
	archiveBrowsing     bool
}

// CreateSqliteAnalyzer creates a new SQLite analyzer
func CreateSqliteAnalyzer(dbPath string) (*SqliteAnalyzer, error) {
	storage, err := NewSqliteStorage(dbPath)
	if err != nil {
		return nil, err
	}

	return &SqliteAnalyzer{
		storage: storage,
		progress: &common.CurrentProgress{
			ItemCount: 0,
			TotalSize: int64(0),
		},
		progressChan:     make(chan common.CurrentProgress, 1),
		progressOutChan:  make(chan common.CurrentProgress, 1),
		progressDoneChan: make(chan struct{}),
		doneChan:         make(common.SignalGroup),
		wait:             (&WaitGroup{}).Init(),
	}, nil
}

// SetFollowSymlinks sets whether symlinks should be followed
func (a *SqliteAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

// SetShowAnnexedSize sets whether to use annexed size
func (a *SqliteAnalyzer) SetShowAnnexedSize(v bool) {
	a.gitAnnexedSize = v
}

// SetTimeFilter sets the time filter function
func (a *SqliteAnalyzer) SetTimeFilter(matchesTimeFilterFn common.TimeFilter) {
	a.matchesTimeFilterFn = matchesTimeFilterFn
}

// SetArchiveBrowsing sets whether archive browsing is enabled
func (a *SqliteAnalyzer) SetArchiveBrowsing(v bool) {
	a.archiveBrowsing = v
}

// SetFileTypeFilter sets the file type filter
func (a *SqliteAnalyzer) SetFileTypeFilter(filter common.ShouldFileBeIgnored) {
	a.ignoreFileType = filter
}

// GetProgressChan returns the progress channel
func (a *SqliteAnalyzer) GetProgressChan() chan common.CurrentProgress {
	return a.progressOutChan
}

// GetDone returns the done signal group
func (a *SqliteAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

// ResetProgress resets the progress state
func (a *SqliteAnalyzer) ResetProgress() {
	a.progress = &common.CurrentProgress{}
	a.progressChan = make(chan common.CurrentProgress, 1)
	a.progressOutChan = make(chan common.CurrentProgress, 1)
	a.progressDoneChan = make(chan struct{})
	a.doneChan = make(common.SignalGroup)
	a.wait = (&WaitGroup{}).Init()
}

// AnalyzeDir analyzes the given path and stores results in SQLite.
// If the database already contains data, it loads from the database instead of re-scanning.
func (a *SqliteAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	// Check if database already has data
	if a.storage.HasData() {
		log.Printf("Loading analysis from existing SQLite database")
		rootItem, err := a.storage.GetRootItem()
		if err != nil {
			log.Printf("Error loading from database, will re-scan: %v", err)
		} else {
			// Signal that we're done immediately
			a.doneChan.Broadcast()
			return rootItem
		}
	}

	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

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

	go a.updateProgress()

	// Process directory and get the root item
	rootItem := a.processDir(path, nil)

	a.wait.Wait()

	// Commit bulk insert transaction
	if err := a.storage.EndBulkInsert(); err != nil {
		log.Printf("Error committing bulk insert: %v", err)
	}

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return rootItem
}

func (a *SqliteAnalyzer) processDir(path string, parentID *int64) *SqliteItem {
	// Start with 4096 for directory's own size/usage, matching Dir.UpdateStats behavior
	var (
		totalSize  int64 = 4096
		totalUsage int64 = 4096
		filesSize  int64 // only files in this directory, for progress reporting
		itemCount  = 1
	)

	a.wait.Add(1)
	defer a.wait.Done()

	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}

	// Get directory info for mtime
	dirInfo, err := os.Stat(path)
	var dirMtime time.Time
	if err == nil {
		dirMtime = dirInfo.ModTime()
	}

	// Insert directory into database (size/usage will be updated later)
	dirID, err := a.storage.InsertItem(
		parentID,
		filepath.Base(path),
		true,
		0, // size will be updated later
		0, // usage will be updated later
		dirMtime,
		1, // item_count will be updated later
		0,
		getDirFlag(err, len(files)),
	)
	if err != nil {
		log.Print(err.Error())
		return nil
	}

	// Process children
	for _, f := range files {
		name := f.Name()
		entryPath := filepath.Join(path, name)

		if f.IsDir() {
			if a.ignoreDir(name, entryPath) {
				continue
			}

			// Process subdirectory recursively
			subItem := a.processDir(entryPath, &dirID)
			if subItem != nil {
				totalSize += subItem.size
				totalUsage += subItem.usage
				itemCount += subItem.itemCount
			}
		} else {
			info, err := f.Info()
			if err != nil {
				log.Print(err.Error())
				continue
			}

			if a.followSymlinks && info.Mode()&os.ModeSymlink != 0 {
				infoF, err := followSymlink(entryPath, a.gitAnnexedSize)
				if err != nil {
					log.Print(err.Error())
					continue
				}
				if infoF != nil {
					info = infoF
				}
			}

			// Apply time filter
			if a.matchesTimeFilterFn != nil && !a.matchesTimeFilterFn(info.ModTime()) {
				continue
			}

			// Apply file type filter
			if a.ignoreFileType != nil && a.ignoreFileType(name) {
				continue
			}

			fileSize := info.Size()
			fileUsage, fileMli := getSyscallStats(info)
			fileFlag := getFlag(info)

			// Handle hard links: if inode already seen, don't count size
			if fileMli != 0 && a.storage.HasInode(fileMli) {
				fileSize = 0
				fileUsage = 0
				fileFlag = 'H'
			}

			_, err = a.storage.InsertItem(
				&dirID,
				name,
				false,
				fileSize,
				fileUsage,
				info.ModTime(),
				1,
				fileMli,
				fileFlag,
			)
			if err != nil {
				log.Print(err.Error())
				continue
			}

			totalSize += fileSize
			totalUsage += fileUsage
			filesSize += fileUsage
			itemCount++
		}
	}

	// Update directory with computed stats
	err = a.storage.UpdateItem(dirID, totalSize, totalUsage, itemCount)
	if err != nil {
		log.Printf("Error updating item: %v", err)
	}

	// Report progress (only files in this dir, subdirs already reported themselves)
	a.progressChan <- common.CurrentProgress{
		CurrentItemName: path,
		ItemCount:       len(files),
		TotalSize:       filesSize,
	}

	// Return SqliteItem for the directory
	return &SqliteItem{
		storage:   a.storage,
		id:        dirID,
		parentID:  parentID,
		name:      filepath.Base(path),
		isDir:     true,
		size:      totalSize,
		usage:     totalUsage,
		mtime:     dirMtime,
		itemCount: itemCount,
		flag:      getDirFlag(err, len(files)),
	}
}

func (a *SqliteAnalyzer) updateProgress() {
	for {
		select {
		case <-a.progressDoneChan:
			return
		case progress := <-a.progressChan:
			a.progress.CurrentItemName = progress.CurrentItemName
			a.progress.ItemCount += progress.ItemCount
			a.progress.TotalSize += progress.TotalSize
		}

		select {
		case a.progressOutChan <- *a.progress:
		default:
		}
	}
}
