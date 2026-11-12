package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
)

func TestReadFromStorageRequiresDatabase(t *testing.T) {
	_, err := runApp(
		&Flags{LogFile: "/dev/null", ReadFromStorage: true},
		[]string{"test_dir"}, false, testdev.DevicesInfoGetterMock{},
	)

	assert.ErrorContains(t, err, "--read-from-storage requires --db")
}

func TestReadFromStorageRequiresExistingDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing.sqlite")

	_, err := runApp(
		&Flags{LogFile: "/dev/null", ReadFromStorage: true, DbPath: dbPath},
		[]string{"test_dir"}, false, testdev.DevicesInfoGetterMock{},
	)

	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.NoFileExists(t, dbPath, "an empty database was created")
}

// --read-from-storage shows the analysis stored by the earlier scan, not the
// current state of the disk, with either database.
func TestReadFromStorageShowsTheStoredAnalysis(t *testing.T) {
	for _, db := range []string{"gdu.sqlite", "gdu.badger"} {
		t.Run(db, func(t *testing.T) {
			fin := testdir.CreateTestDir()
			defer fin()
			dbPath := filepath.Join(t.TempDir(), db)
			run := func(readFromStorage bool) string {
				t.Helper()
				out, err := runApp(
					&Flags{LogFile: "/dev/null", DbPath: dbPath, ReadFromStorage: readFromStorage},
					[]string{"test_dir"}, false, testdev.DevicesInfoGetterMock{},
				)
				require.NoError(t, err)
				return out
			}

			scanned := run(false)
			require.Contains(t, scanned, "nested")
			require.NoError(t, os.WriteFile("test_dir/nested/added", make([]byte, 1<<20), 0o600))

			assert.Equal(t, scanned, run(true))
			assert.NotEqual(t, scanned, run(false), "a scan without -r does not rescan")
		})
	}
}

func TestReadFromStorageOfAnotherPath(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	dbPath := filepath.Join(t.TempDir(), "gdu.sqlite")

	_, err := runApp(
		&Flags{LogFile: "/dev/null", DbPath: dbPath},
		[]string{"test_dir"}, false, testdev.DevicesInfoGetterMock{},
	)
	require.NoError(t, err)

	_, err = runApp(
		&Flags{LogFile: "/dev/null", DbPath: dbPath, ReadFromStorage: true},
		[]string{"test_dir/nested"}, false, testdev.DevicesInfoGetterMock{},
	)

	assert.ErrorContains(t, err, "holds the analysis of")
}
