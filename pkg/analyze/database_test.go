package analyze

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/internal/testdir"
)

// databaseBackend is a Database to run the contract tests on.
type databaseBackend struct {
	name string
	// open opens the database in the file path, and returns it with the
	// function closing it
	open func(t *testing.T, path string) (Database, func())
	// storeUnrecorded stores an analysis of path the way gdu did before it
	// recorded analysisInfo
	storeUnrecorded func(t *testing.T, db Database, path string)
}

var databaseBackends = []databaseBackend{
	{
		name: "badger",
		open: func(_ *testing.T, path string) (Database, func()) {
			return createStoredAnalyzer(path), func() {}
		},
		storeUnrecorded: func(t *testing.T, db Database, path string) {
			storage := db.(*StoredAnalyzer).storage
			require.NoError(t, storage.storeRecord(path, &storedRecord{ItemCount: 1}))
		},
	},
	{
		name: "sqlite",
		open: func(t *testing.T, path string) (Database, func()) {
			a, err := createSqliteAnalyzer(path)
			require.NoError(t, err)
			return a, func() { a.storage.Close() }
		},
		storeUnrecorded: func(t *testing.T, db Database, path string) {
			storage := db.(*SqliteAnalyzer).storage
			require.NoError(t, storage.SetMetadata("top_dir_path", path))
			_, err := storage.InsertItem(nil, filepath.Base(path), true, 0, 0, time.Now(), 1, 0, ' ')
			require.NoError(t, err)
		},
	},
}

// withInfo calls f with the record of what db holds, open.
func withInfo(db Database, f func(infoStore)) {
	switch db := db.(type) {
	case *StoredAnalyzer:
		f(db.storage)
	case *SqliteAnalyzer:
		f(db.storage)
	}
}

func formatOf(db Database) int {
	if _, ok := db.(*StoredAnalyzer); ok {
		return storedFormat
	}
	return sqliteFormat
}

// scanInto scans path into the database in the file dbPath as gdu does,
// computing the totals, with ctx.
func scanInto(ctx context.Context, t *testing.T, backend databaseBackend, dbPath, path string) {
	t.Helper()
	db, closeFn := backend.open(t, dbPath)
	defer closeFn()
	root := db.AnalyzeDir(ctx, path, Options{})
	require.NotNil(t, root)
	root.UpdateStats(false)
}

// load loads the analysis of path from the database in the file dbPath,
// opened anew as by a later run of gdu.
func load(t *testing.T, backend databaseBackend, dbPath, path string) (Node, error) {
	t.Helper()
	db, closeFn := backend.open(t, dbPath)
	t.Cleanup(closeFn)
	return db.LoadAnalysis(path)
}

func forEachDatabase(t *testing.T, test func(t *testing.T, backend databaseBackend, dbPath string)) {
	for _, backend := range databaseBackends {
		t.Run(backend.name, func(t *testing.T) {
			fin := testdir.CreateTestDir()
			defer fin()
			test(t, backend, filepath.Join(t.TempDir(), "gdu."+backend.name))
		})
	}
}

func TestDatabaseLoadsTheCompleteAnalysis(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, backend databaseBackend, dbPath string) {
		scanInto(context.Background(), t, backend, dbPath, "test_dir")

		root, err := load(t, backend, dbPath, "test_dir")

		require.NoError(t, err)
		assert.Equal(t, "test_dir", root.GetName())
		assert.Equal(t, int64(5), root.GetItemCount())
		assert.Equal(t, int64(7), root.GetSize())
	})
}

func TestDatabaseHoldsNoAnalysisOfAnotherPath(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, backend databaseBackend, dbPath string) {
		scanInto(context.Background(), t, backend, dbPath, "test_dir")

		_, err := load(t, backend, dbPath, "test_dir/nested")

		assert.ErrorIs(t, err, ErrNoAnalysis)
		assert.ErrorContains(t, err, "holds no analysis of test_dir/nested, it holds the analysis of test_dir")
	})
}

// A scan replaces what the database held.
func TestDatabaseHoldsTheLastScan(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, backend databaseBackend, dbPath string) {
		scanInto(context.Background(), t, backend, dbPath, "test_dir")
		scanInto(context.Background(), t, backend, dbPath, "test_dir/nested")

		_, err := load(t, backend, dbPath, "test_dir")
		assert.ErrorIs(t, err, ErrNoAnalysis)
		root, err := load(t, backend, dbPath, "test_dir/nested")
		require.NoError(t, err)
		assert.Equal(t, int64(4), root.GetItemCount())
	})
}

func TestEmptyDatabaseHoldsNoAnalysis(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, backend databaseBackend, dbPath string) {
		_, err := load(t, backend, dbPath, "test_dir")

		assert.ErrorIs(t, err, ErrNoAnalysis)
		assert.ErrorContains(t, err, "holds no analysis of test_dir")
	})
}

// A stopped scan keeps what it found, in the database as in memory.
func TestDatabaseLoadsAStoppedAnalysis(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, backend databaseBackend, dbPath string) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		scanInto(ctx, t, backend, dbPath, "test_dir")

		_, err := load(t, backend, dbPath, "test_dir")

		require.NoError(t, err)
		db, closeFn := backend.open(t, dbPath)
		defer closeFn()
		withInfo(db, func(store infoStore) {
			info, ok, err := store.readInfo()
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, analysisStopped, info.State)
		})
	})
}

// A scan which never ended, as when gdu was killed, leaves an analysis which
// is not loaded: the database may hold part of the old one and part of the
// new one.
func TestDatabaseRefusesAnUnfinishedAnalysis(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, backend databaseBackend, dbPath string) {
		scanInto(context.Background(), t, backend, dbPath, "test_dir")
		db, closeFn := backend.open(t, dbPath)
		withInfo(db, func(store infoStore) {
			require.NoError(t, beginAnalysis(store, formatOf(db), "test_dir"))
		})
		closeFn()

		_, err := load(t, backend, dbPath, "test_dir")

		assert.ErrorIs(t, err, ErrIncompleteAnalysis)
	})
}

func TestDatabaseRefusesAnotherFormat(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, backend databaseBackend, dbPath string) {
		scanInto(context.Background(), t, backend, dbPath, "test_dir")
		db, closeFn := backend.open(t, dbPath)
		withInfo(db, func(store infoStore) {
			info := analysisInfo{Format: formatOf(db) + 1, Root: "test_dir", State: analysisComplete}
			require.NoError(t, store.writeInfo(info))
		})
		closeFn()

		_, err := load(t, backend, dbPath, "test_dir")

		assert.ErrorIs(t, err, ErrStoredFormat)
	})
}

// A database written before gdu recorded its analysis is of another format;
// whether its scan finished is unknown.
func TestDatabaseRefusesAnUnrecordedAnalysis(t *testing.T) {
	forEachDatabase(t, func(t *testing.T, backend databaseBackend, dbPath string) {
		db, closeFn := backend.open(t, dbPath)
		backend.storeUnrecorded(t, db, "test_dir")
		closeFn()

		_, err := load(t, backend, dbPath, "test_dir")
		assert.ErrorIs(t, err, ErrStoredFormat)

		_, err = load(t, backend, dbPath, "other_dir")
		assert.ErrorIs(t, err, ErrNoAnalysis)
	})
}

// Badger stores the totals when UpdateStats computes them, so a scan whose
// totals were never computed is unfinished.
func TestBadgerAnalysisWithoutTotalsIsUnfinished(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	dbPath := filepath.Join(t.TempDir(), "gdu.badger")
	createStoredAnalyzer(dbPath).AnalyzeDir(context.Background(), "test_dir", Options{})

	_, err := createStoredAnalyzer(dbPath).LoadAnalysis("test_dir")

	assert.ErrorIs(t, err, ErrIncompleteAnalysis)
}

// fakeInfoStore records the analysis in memory.
type fakeInfoStore struct {
	info analysisInfo
	ok   bool
}

func (f *fakeInfoStore) writeInfo(info analysisInfo) error {
	f.info, f.ok = info, true
	return nil
}

func (f *fakeInfoStore) readInfo() (analysisInfo, bool, error) { return f.info, f.ok, nil }

func (f *fakeInfoStore) holdsUnrecorded(string) (bool, error) { return false, nil }

// A scan which failed to store part of the analysis is not taken for
// complete.
func TestFailedWritesLeaveTheAnalysisIncomplete(t *testing.T) {
	store := &fakeInfoStore{}
	require.NoError(t, beginAnalysis(store, 1, "/dir"))

	assert.Error(t, endAnalysis(store, 1, "/dir", false, true))

	assert.ErrorIs(t, checkAnalysis(store, 1, "/dir"), ErrIncompleteAnalysis)
	require.NoError(t, endAnalysis(store, 1, "/dir", false, false))
	assert.NoError(t, checkAnalysis(store, 1, "/dir"))
}

func TestCorruptInfoIsAnotherFormat(t *testing.T) {
	_, err := unmarshalInfo([]byte("{"))

	assert.ErrorIs(t, err, ErrStoredFormat)
}
