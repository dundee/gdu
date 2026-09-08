//go:build linux

package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestNoCrossWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", NoCross: true},
		[]string{"test_dir"},
		false,
		device.LinuxDevicesInfoGetter{MountsPath: "/xxxyyy"},
	)

	assert.Equal(t, "loading mount points: open /xxxyyy: no such file or directory", err.Error())
	assert.Empty(t, out)
}

func TestListDevicesWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	_, err := runApp(
		&Flags{LogFile: "/dev/null", ShowDisks: true},
		[]string{},
		false,
		device.LinuxDevicesInfoGetter{MountsPath: "/xxxyyy"},
	)

	assert.Equal(t, "loading mount points: open /xxxyyy: no such file or directory", err.Error())
}

func TestOutputFileError(t *testing.T) {
	out, err := runApp(
		&Flags{LogFile: "/dev/null", OutputFile: "/xyzxyz"},
		[]string{},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestUseStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	const storagePath = "/tmp/badger-test.badger"
	defer func() {
		err := os.RemoveAll(storagePath)
		if err != nil {
			panic(err)
		}
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", DbPath: storagePath},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestReadFromStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	storagePath := "/tmp/badger-test4.badger"
	defer func() {
		err := os.RemoveAll(storagePath)
		if err != nil {
			panic(err)
		}
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", DbPath: storagePath},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)
	assert.Contains(t, out, "nested")
	assert.Nil(t, err)

	out, err = runApp(
		&Flags{LogFile: "/dev/null", ReadFromStorage: true, DbPath: storagePath},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)
	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestAnalyzePathWithSqliteStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	dbPath := filepath.Join(t.TempDir(), "db", "test.sqlite")

	out, err := runApp(
		&Flags{LogFile: "/dev/null", DbPath: dbPath},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)
	assert.Contains(t, out, "nested")
	assert.Nil(t, err)

	out, err = runApp(
		&Flags{LogFile: "/dev/null", DbPath: dbPath, ReadFromStorage: true},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)
	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestAnalyzePathWithSqliteStorageError(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	parentFile := filepath.Join(t.TempDir(), "parent-file")
	err := os.WriteFile(parentFile, []byte("x"), 0o600)
	assert.Nil(t, err)

	out, err := runApp(
		&Flags{LogFile: "/dev/null", DbPath: filepath.Join(parentFile, "db.sqlite")},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.Empty(t, out)
	assert.ErrorContains(t, err, "creating sqlite analyzer")
}

func TestNoCrossWithMountInfo(t *testing.T) {
	temp := t.TempDir()
	root := filepath.Join(temp, "scan")
	for name, size := range map[string]int{"root.dat": 101, "same/same.dat": 203, "foreign/foreign.dat": 307} {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o600))
	}
	var stat unix.Stat_t
	require.NoError(t, unix.Stat(root, &stat))
	major, minor := unix.Major(uint64(stat.Dev)), unix.Minor(uint64(stat.Dev))
	rootField := strings.ReplaceAll(root, " ", `\040`)
	mountsPath := filepath.Join(temp, "mountinfo")
	mounts := fmt.Sprintf(`1 1 %d:%d / / rw - ext4 /dev/example rw
2 1 %d:%d /store %s/same ro - ext4 /dev/example rw
3 1 %d:%d / %s/foreign rw - tmpfs tmpfs rw
`, major, minor, major, minor, rootField, major, minor^1, rootField)
	require.NoError(t, os.WriteFile(mountsPath, []byte(mounts), 0o600))

	for _, tt := range []struct {
		name    string
		noCross bool
		ignore  []string
		want    string
	}{
		{name: "unrestricted", want: "611"},
		{name: "same filesystem", noCross: true, want: "304"},
		{name: "explicit ignore", noCross: true, ignore: []string{filepath.Join(root, "same")}, want: "101"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			app := App{
				Flags: &Flags{
					LogFile: "/dev/null", NoCross: tt.noCross, IgnoreDirs: tt.ignore,
					ShowApparentSize: true, NoPrefix: true, NoColor: true, NoProgress: true, Depth: 3,
				},
				Args: []string{root}, Writer: &output,
				TermApp: testapp.CreateMockedApp(false), PathChecker: os.Stat,
				Getter: device.LinuxDevicesInfoGetter{MountsPath: mountsPath},
			}
			require.NoError(t, app.Run())
			require.NotEmpty(t, output.String())
			assert.Equal(t, tt.want, strings.Fields(output.String())[0])
			assert.Equal(t, !tt.noCross, strings.Contains(output.String(), "/foreign/foreign.dat"))
			assert.Equal(t, len(tt.ignore) == 0, strings.Contains(output.String(), "/same/same.dat"))
		})
	}
}
