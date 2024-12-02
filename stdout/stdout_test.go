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

	ui := CreateStdoutUI(output, false, false, false, false, false, true, false, false, 0)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "nested")
}

func TestShowSummary(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, true, false, true, false, true, false, false, false, 0)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "test_dir")
}

func TestShowSummaryBw(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, false, false, false, false, true, false, false, false, 0)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "test_dir")
}

func TestShowTop(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, true, false, true, false, true, false, false, false, 2)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "test_dir/nested/subnested/file")
	assert.Contains(t, output.String(), "test_dir/nested/file2")
}

func TestShowTopBw(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, false, false, false, false, true, false, false, false, 2)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "test_dir/nested/subnested/file")
	assert.Contains(t, output.String(), "test_dir/nested/file2")
}

func TestAnalyzeSubdir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, false, false, false, false, false, false, false, false, 0)
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

	ui := CreateStdoutUI(output, true, false, true, false, false, false, false, false, 0)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir/nested", nil)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "subnested")
}

func TestAnalyzePathWoUnicode(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, false, true, true, false, false, false, false, false, 0)
	ui.UseOldProgressRunes()
	err := ui.AnalyzePath("test_dir/nested", nil)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "subnested")
}

func TestItemRows(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true, false, false, false, false, false, false, 0)
	ui.Analyzer = &testanalyze.MockedAnalyzer{}
	err := ui.AnalyzePath("test_dir", nil)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "KiB")
}

func TestAnalyzePathWithProgress(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true, true, false, false, false, false, false, 0)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "nested")
}

func TestShowDevices(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true, false, false, false, false, false, false, 0)
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "Device")
	assert.Contains(t, output.String(), "xxx")
}

func TestShowDevicesWithColor(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true, false, false, false, false, false, 0)
	err := ui.ListDevices(getDevicesInfoMock())

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "Device")
	assert.Contains(t, output.String(), "xxx")
}

func TestReadAnalysisWithColor(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/test.json", os.O_RDONLY, 0o644)
	assert.Nil(t, err)

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true, false, false, false, false, false, 0)
	err = ui.ReadAnalysis(input)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "main.go")
}

func TestReadAnalysisBw(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/test.json", os.O_RDONLY, 0o644)
	assert.Nil(t, err)

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, false, false, false, false, false, false, false, 0)
	err = ui.ReadAnalysis(input)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), "main.go")
}

func TestReadAnalysisWithWrongFile(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/wrong.json", os.O_RDONLY, 0o644)
	assert.Nil(t, err)

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true, false, false, false, false, false, 0)
	err = ui.ReadAnalysis(input)

	assert.NotNil(t, err)
}

func TestReadAnalysisWithSummarize(t *testing.T) {
	input, err := os.OpenFile("../internal/testdata/test.json", os.O_RDONLY, 0o644)
	assert.Nil(t, err)

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, false, false, false, true, false, false, false, 0)
	err = ui.ReadAnalysis(input)

	assert.Nil(t, err)
	assert.Contains(t, output.String(), " gdu\n")
}

func TestMaxInt(t *testing.T) {
	assert.Equal(t, 5, maxInt(2, 5))
	assert.Equal(t, 4, maxInt(4, 2))
}

func TestFormatSize(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true, false, false, false, false, false, 0)

	assert.Contains(t, ui.formatSize(1), "B")
	assert.Contains(t, ui.formatSize(1<<10+1), "KiB")
	assert.Contains(t, ui.formatSize(1<<20+1), "MiB")
	assert.Contains(t, ui.formatSize(1<<30+1), "GiB")
	assert.Contains(t, ui.formatSize(1<<40+1), "TiB")
	assert.Contains(t, ui.formatSize(1<<50+1), "PiB")
	assert.Contains(t, ui.formatSize(1<<60+1), "EiB")
}

func TestFormatSizeDec(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true, false, false, false, true, false, 0)

	assert.Contains(t, ui.formatSize(1), "B")
	assert.Contains(t, ui.formatSize(1<<10+1), "kB")
	assert.Contains(t, ui.formatSize(1<<20+1), "MB")
	assert.Contains(t, ui.formatSize(1<<30+1), "GB")
	assert.Contains(t, ui.formatSize(1<<40+1), "TB")
	assert.Contains(t, ui.formatSize(1<<50+1), "PB")
	assert.Contains(t, ui.formatSize(1<<60+1), "EB")
}

func TestFormatSizeRaw(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, true, true, true, false, false, false, true, true, 0)

	assert.Equal(t, ui.formatSize(1), "1")
	assert.Equal(t, ui.formatSize(1<<10+1), "1025")
	assert.Equal(t, ui.formatSize(1<<20+1), "1048577")
	assert.Equal(t, ui.formatSize(1<<30+1), "1073741825")
	assert.Equal(t, ui.formatSize(1<<40+1), "1099511627777")
	assert.Equal(t, ui.formatSize(1<<50+1), "1125899906842625")
	assert.Equal(t, ui.formatSize(1<<60+1), "1152921504606846977")
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
