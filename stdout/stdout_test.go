package stdout

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/analyze"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzePath(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, false, false)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.AnalyzePath("test_dir", analyze.ProcessDir)

	assert.Contains(t, output.String(), "nested")
}

func TestAnalyzePathWithColors(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	buff := make([]byte, 10)
	output := bytes.NewBuffer(buff)

	ui := CreateStdoutUI(output, true, false)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.AnalyzePath("test_dir/nested", analyze.ProcessDir)

	assert.Contains(t, output.String(), "subnested")
}

func TestAnalyzePathWithProgress(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	output := bytes.NewBuffer(make([]byte, 10))

	ui := CreateStdoutUI(output, false, true)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	ui.AnalyzePath("test_dir", analyze.ProcessDir)

	assert.Contains(t, output.String(), "nested")
}

func printBuffer(buff *bytes.Buffer) {
	for i, x := range buff.String() {
		println(i, string(x))
	}
}
