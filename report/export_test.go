package report

import (
	"bytes"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"

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

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, false)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
}

func TestAnalyzePathWithProgress(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, true, true)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
}

func TestShowDevices(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, true)
	err := ui.ListDevices(device.Getter)

	assert.Contains(t, err.Error(), "not supported")
}

func TestReadAnalysisWhileExporting(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, true)
	err := ui.ReadAnalysis(output)

	assert.Contains(t, err.Error(), "not possible while exporting")
}

func TestExportToFile(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	reportOutput, err := os.OpenFile("output.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	assert.Nil(t, err)
	defer func() {
		os.Remove("output.json")
	}()

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, true)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err = ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.StartUILoop()
	assert.Nil(t, err)

	reportOutput, err = os.OpenFile("output.json", os.O_RDONLY, 0644)
	assert.Nil(t, err)
	_, err = reportOutput.Seek(0, 0)
	assert.Nil(t, err)
	buff := make([]byte, 200)
	_, err = reportOutput.Read(buff)
	assert.Nil(t, err)

	assert.Contains(t, string(buff), `"name":"nested"`)
}

func TestFormatSize(t *testing.T) {
	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, true)

	assert.Contains(t, ui.formatSize(1), "B")
	assert.Contains(t, ui.formatSize(1<<10+1), "KiB")
	assert.Contains(t, ui.formatSize(1<<20+1), "MiB")
	assert.Contains(t, ui.formatSize(1<<30+1), "GiB")
	assert.Contains(t, ui.formatSize(1<<40+1), "TiB")
	assert.Contains(t, ui.formatSize(1<<50+1), "PiB")
	assert.Contains(t, ui.formatSize(1<<60+1), "EiB")
}
