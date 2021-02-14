package stdout

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/analyze"
	"github.com/dundee/gdu/device"
	"github.com/dundee/gdu/internal/testanalyze"
	"github.com/dundee/gdu/internal/testdev"
	"github.com/dundee/gdu/internal/testdir"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzePath(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, false, false, false)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)
	ui.StartUILoop()

	assert.Contains(t, output.String(), "nested")
}

func TestAnalyzePathWithColors(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, true, false, true)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.AnalyzePath("test_dir/nested", analyze.ProcessDir, nil)

	assert.Contains(t, output.String(), "subnested")
}

func TestItemRows(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true, false)
	ui.AnalyzePath("test_dir", testanalyze.MockedProcessDir, nil)

	assert.Contains(t, output.String(), "TiB")
	assert.Contains(t, output.String(), "GiB")
	assert.Contains(t, output.String(), "MiB")
	assert.Contains(t, output.String(), "KiB")
}

func TestAnalyzePathWithProgress(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true, true)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.AnalyzePath("test_dir", analyze.ProcessDir, nil)

	assert.Contains(t, output.String(), "nested")
}

func TestShowDevices(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true, false)
	ui.ListDevices(getDevicesInfoMock())

	assert.Contains(t, output.String(), "Device")
	assert.Contains(t, output.String(), "xxx")
}

func TestShowDevicesWithColor(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true)
	ui.ListDevices(getDevicesInfoMock())

	assert.Contains(t, output.String(), "Device")
	assert.Contains(t, output.String(), "xxx")
}

func TestShowDevicesWithErr(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	getter := device.LinuxDevicesInfoGetter{MountsPath: "/xyzxyz"}
	ui := CreateStdoutUI(output, false, true, false)
	err := ui.ListDevices(getter)

	assert.Contains(t, err.Error(), "no such file")
}

func TestMaxInt(t *testing.T) {
	assert.Equal(t, 5, maxInt(2, 5))
	assert.Equal(t, 4, maxInt(4, 2))
}

func printBuffer(buff *bytes.Buffer) {
	for i, x := range buff.String() {
		println(i, string(x))
	}
}

func getDevicesInfoMock() device.DevicesInfoGetter {
	item := &device.Device{
		Name: "xxx",
	}

	mock := testdev.DevicesInfoGetterMock{}
	mock.Devices = []*device.Device{item}
	return mock
}
