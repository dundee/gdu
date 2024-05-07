//go:build linux
// +build linux

package report

import (
	"bytes"
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/stretchr/testify/assert"
)

func TestReadFromStorage(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	const storagePath = "/tmp/badger-test2"
	defer func() {
		err := os.RemoveAll(storagePath)
		if err != nil {
			panic(err)
		}
	}()

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, true, false, false)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.SetAnalyzer(analyze.CreateStoredAnalyzer(storagePath))
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.ReadFromStorage(storagePath, "test_dir")

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
}

func TestReadFromStorageWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	const storagePath = "/tmp/badger-test3"

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, false, false, false, false)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	err := ui.ReadFromStorage(storagePath, "test_dir")

	assert.ErrorContains(t, err, "Key not found")
}
