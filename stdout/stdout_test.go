package stdout

import (
	"bytes"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/testanalyze"
	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestAnalyzePath(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, false, false, false)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "nested")
}

func TestAnalyzeSubdir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, false, false, false)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir/nested", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "file2")
}

func TestAnalyzePathWithColors(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, true, false, true)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir/nested", nil)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "subnested")
}

func TestItemRows(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true, false)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	err := ui.AnalyzePath("test_dir", nil)

	assert.Nil(t, err)
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
	err := ui.AnalyzePath("test_dir", nil)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "nested")
}

func TestShowDevices(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true, false)
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "Device")
	assert.Contains(t, output.String(), "xxx")
}

func TestShowDevicesWithColor(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true)
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "Device")
	assert.Contains(t, output.String(), "xxx")
}

func TestReadAnalysisWithColor(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/test.json", os.O_RDONLY, 0644)
	assert.Nil(t, err)

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true)
	err = ui.ReadAnalysis(input)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "main.go")
}

func TestReadAnalysisBw(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/test.json", os.O_RDONLY, 0644)
	assert.Nil(t, err)

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, false, false)
	err = ui.ReadAnalysis(input)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "main.go")
}

func TestReadAnalysisWithWrongFile(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/wrong.json", os.O_RDONLY, 0644)
	assert.Nil(t, err)

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true)
	err = ui.ReadAnalysis(input)

	assert.NotNil(t, err)
}

func TestMaxInt(t *testing.T) {
	assert.Equal(t, 5, maxInt(2, 5))
	assert.Equal(t, 4, maxInt(4, 2))
}

func TestFormatSize(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true)

	assert.Contains(t, ui.formatSize(1), "B")
	assert.Contains(t, ui.formatSize(1<<10+1), "KiB")
	assert.Contains(t, ui.formatSize(1<<20+1), "MiB")
	assert.Contains(t, ui.formatSize(1<<30+1), "GiB")
	assert.Contains(t, ui.formatSize(1<<40+1), "TiB")
	assert.Contains(t, ui.formatSize(1<<50+1), "PiB")
	assert.Contains(t, ui.formatSize(1<<60+1), "EiB")
}

// func printBuffer(buff *bytes.Buffer) {
// 	for i, x := range buff.String() {
// 		println(i, string(x))
// 	}
// }

func getDevicesInfoMock() device.DevicesInfoGetter {
	item := &device.Device{
		Name: "xxx",
	}

	mock := testdev.DevicesInfoGetterMock{}
	mock.Devices = []*device.Device{item}
	return mock
}
