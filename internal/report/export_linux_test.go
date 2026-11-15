//go:build linux

package report

import (
	"bytes"
	"os"
	"testing"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/session"
	"github.com/dundee/gdu/v6/internal/testdir"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/analyze/badger"
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

	ui := CreateExportUI(output, reportOutput, analyze.CreateAnalyzer(), &config.Settings{Display: config.Display{Progress: true}})
	ui.Analyzer = (badger.CreateAnalyzer(storagePath))
	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)
	err = ui.ReadFromStorage("test_dir")

	assert.Nil(t, err)
	assert.Contains(t, reportOutput.String(), `"name":"nested"`)
}

func TestReadFromStorageWithErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	const storagePath = "/tmp/badger-test3"

	output := bytes.NewBuffer(make([]byte, 10))
	reportOutput := bytes.NewBuffer(make([]byte, 10))

	ui := CreateExportUI(output, reportOutput, analyze.CreateAnalyzer(), &config.Settings{})
	ui.Analyzer = (badger.CreateAnalyzer(storagePath))
	err := ui.ReadFromStorage("test_dir")

	assert.ErrorContains(t, err, "holds no analysis of test_dir")
}

func TestReadFromStorageNeedsDatabase(t *testing.T) {
	ui := CreateExportUI(&bytes.Buffer{}, &bytes.Buffer{}, analyze.CreateAnalyzer(), &config.Settings{})

	err := ui.ReadFromStorage("test_dir")

	assert.ErrorIs(t, err, session.ErrNoDatabase)
}
