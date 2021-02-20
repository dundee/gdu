package run

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/device"
	"github.com/dundee/gdu/internal/testapp"
	"github.com/dundee/gdu/internal/testdev"
	"github.com/dundee/gdu/internal/testdir"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(
		&RunFlags{ShowVersion: true},
		[]string{},
		false,
		buff,
		testapp.CreateMockedApp(false),
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, buff.String(), "Version:\t development")
}

func TestLogError(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 10))
	err := Run(
		&RunFlags{LogFile: "/xyzxyz"},
		[]string{},
		false,
		buff,
		testapp.CreateMockedApp(false),
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, err.Error(), "permission denied")
}

func TestAnalyzePath(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(
		&RunFlags{LogFile: "/dev/null"},
		[]string{"test_dir"},
		false,
		buff,
		testapp.CreateMockedApp(false),
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, buff.String(), "nested")
}

func TestAnalyzePathWithGui(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(
		&RunFlags{LogFile: "/dev/null"},
		[]string{"test_dir"},
		true,
		buff,
		testapp.CreateMockedApp(false), testdev.DevicesInfoGetterMock{},
	)
}

func TestNoCross(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(
		&RunFlags{LogFile: "/dev/null", NoCross: true},
		[]string{"test_dir"},
		false,
		buff,
		testapp.CreateMockedApp(false),
		testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, buff.String(), "nested")
}

func TestNoCrossWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	getter := device.LinuxDevicesInfoGetter{MountsPath: "/xxxyyy"}
	err := Run(
		&RunFlags{LogFile: "/dev/null", NoCross: true},
		[]string{"test_dir"},
		false,
		buff,
		testapp.CreateMockedApp(false),
		getter,
	)

	assert.Equal(t, "Error loading mount points: open /xxxyyy: no such file or directory", err.Error())
}

func TestListDevices(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(
		&RunFlags{LogFile: "/dev/null", ShowDisks: true},
		nil,
		false,
		buff,
		testapp.CreateMockedApp(false), testdev.DevicesInfoGetterMock{},
	)

	assert.Contains(t, buff.String(), "Device")
}

func TestListDevicesWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))
	getter := device.LinuxDevicesInfoGetter{MountsPath: "/xxxyyy"}

	err := Run(
		&RunFlags{LogFile: "/dev/null", ShowDisks: true},
		nil,
		false,
		buff,
		testapp.CreateMockedApp(false),
		getter,
	)

	assert.Equal(t, "Error loading mount points: open /xxxyyy: no such file or directory", err.Error())
}

func TestListDevicesWithGui(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(
		&RunFlags{LogFile: "/dev/null", ShowDisks: true},
		nil,
		true,
		buff,
		testapp.CreateMockedApp(false),
		testdev.DevicesInfoGetterMock{},
	)
}
