package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --db names a file to replace; anything gdu did not create there must stay.
func TestDatabaseOtherFilesAreKept(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	dir := t.TempDir()

	notes := filepath.Join(dir, "notes.md")
	require.NoError(t, os.WriteFile(notes, []byte("my notes"), 0o600))

	projects := filepath.Join(dir, "projects.badger")
	require.NoError(t, os.MkdirAll(filepath.Join(projects, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projects, "MANIFEST"), []byte("x"), 0o600))

	notADirectory := filepath.Join(dir, "file.badger")
	require.NoError(t, os.WriteFile(notADirectory, []byte("x"), 0o600))

	foreign := filepath.Join(dir, "app.db")
	db, err := sql.Open("sqlite", foreign)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT); CREATE TABLE users (id INTEGER)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	foreignBefore, err := os.ReadFile(foreign)
	require.NoError(t, err)

	for _, dbPath := range []string{notes, projects, notADirectory, foreign} {
		for _, flags := range []Flags{
			{DbPath: dbPath},
			{DbPath: dbPath, ReadFromStorage: true},
			{DbPath: dbPath, ShowDisks: true},
		} {
			flags.LogFile = "/dev/null"
			_, err := runApp(&flags, []string{"test_dir"}, false, testdev.DevicesInfoGetterMock{})

			assert.ErrorContains(t, err, "refusing to use "+dbPath, "%+v", flags)
		}
	}

	content, err := os.ReadFile(notes)
	require.NoError(t, err)
	assert.Equal(t, "my notes", string(content))
	assert.DirExists(t, filepath.Join(projects, "src"))
	assert.FileExists(t, notADirectory)
	foreignAfter, err := os.ReadFile(foreign)
	require.NoError(t, err)
	assert.Equal(t, foreignBefore, foreignAfter, "the foreign database must not be changed")
}

// A database gdu created is replaced by the next scan into it, and kept by
// runs that do not scan into it.
func TestDatabaseOfGduIsReplacedOnlyByAScan(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	for _, name := range []string{"gdu.badger", "gdu.sqlite"} {
		t.Run(name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), name)
			scan := func(flags Flags) error {
				flags.LogFile = "/dev/null"
				flags.DbPath = dbPath
				_, err := runApp(&flags, []string{"test_dir"}, false, testdev.DevicesInfoGetterMock{})
				return err
			}

			require.NoError(t, os.RemoveAll("test_dir/added"))
			require.NoError(t, scan(Flags{}))
			require.NoError(t, os.MkdirAll("test_dir/added", 0o755))

			// neither reading a report nor a sequential scan touches it
			report := filepath.Join(t.TempDir(), "report.json")
			require.NoError(t, os.WriteFile(report,
				[]byte(`[1,2,{"progname":"gdu","progver":"development","timestamp":1626806293},[{"name":"/other"}]]`), 0o600))
			require.NoError(t, scan(Flags{InputFile: report}))
			require.NoError(t, scan(Flags{SequentialScanning: true}))
			out, err := runApp(&Flags{LogFile: "/dev/null", DbPath: dbPath, ReadFromStorage: true},
				[]string{"test_dir"}, false, testdev.DevicesInfoGetterMock{})
			require.NoError(t, err)
			assert.NotContains(t, out, "added")

			// scanning again replaces it
			require.NoError(t, scan(Flags{}))
			out, err = runApp(&Flags{LogFile: "/dev/null", DbPath: dbPath, ReadFromStorage: true},
				[]string{"test_dir"}, false, testdev.DevicesInfoGetterMock{})
			require.NoError(t, err)
			assert.Contains(t, out, "added")
		})
	}
}

// An empty file or directory, as a user may create to hold the database, is
// used.
func TestDatabaseEmptyPathIsUsed(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	dir := t.TempDir()

	emptyFile := filepath.Join(dir, "empty.sqlite")
	require.NoError(t, os.WriteFile(emptyFile, nil, 0o600))
	emptyDir := filepath.Join(dir, "empty.badger")
	require.NoError(t, os.Mkdir(emptyDir, 0o755))

	for _, dbPath := range []string{emptyFile, emptyDir} {
		out, err := runApp(&Flags{LogFile: "/dev/null", DbPath: dbPath}, []string{"test_dir"}, false, testdev.DevicesInfoGetterMock{})

		require.NoError(t, err, dbPath)
		assert.Contains(t, out, "nested")
	}
}

func TestReadOnlyURI(t *testing.T) {
	uri := readOnlyURI(filepath.Join(t.TempDir(), "a db#1?.sqlite"))

	db, err := sql.Open("sqlite", uri)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE t (a)`)
	assert.Error(t, err, "the database must be opened read-only")
}
