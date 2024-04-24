//go:build linux
// +build linux

package app

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/stretchr/testify/assert"
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

	const storagePath = "/tmp/badger-test"
	defer func() {
		err := os.RemoveAll(storagePath)
		if err != nil {
			panic(err)
		}
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", UseStorage: true, StoragePath: storagePath},
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

	storagePath := "/tmp/badger-test4"
	defer func() {
		err := os.RemoveAll(storagePath)
		if err != nil {
			panic(err)
		}
	}()

	out, err := runApp(
		&Flags{LogFile: "/dev/null", UseStorage: true, StoragePath: storagePath},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)
	assert.Contains(t, out, "nested")
	assert.Nil(t, err)

	out, err = runApp(
		&Flags{LogFile: "/dev/null", ReadFromStorage: true, StoragePath: storagePath},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)
	assert.Contains(t, out, "nested")
	assert.Nil(t, err)
}

func TestReadFromStorageWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	_, err := runApp(
		&Flags{LogFile: "/dev/null", ReadFromStorage: true, StoragePath: "/tmp/badger-xxx"},
		[]string{"test_dir"},
		false,
		testdev.DevicesInfoGetterMock{},
	)

	assert.ErrorContains(t, err, "Key not found")
}
