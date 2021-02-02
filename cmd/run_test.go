package cmd

import (
	"bytes"
	"testing"

	testDir "github.com/dundee/gdu/internal/testing"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(&RunFlags{ShowVersion: true}, []string{}, false, buff, true)

	assert.Contains(t, buff.String(), "Version:\t development")
}

func TestAnalyzePath(t *testing.T) {
	fin := testDir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(&RunFlags{LogFile: "/dev/null"}, []string{"test_dir"}, false, buff, true)

	assert.Contains(t, buff.String(), "nested")
}

func TestAnalyzePathWithGui(t *testing.T) {
	fin := testDir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(&RunFlags{LogFile: "/dev/null"}, []string{"test_dir"}, true, buff, true)
}

func TestListDevices(t *testing.T) {
	fin := testDir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(&RunFlags{LogFile: "/dev/null", ShowDisks: true}, nil, false, buff, true)

	assert.Contains(t, buff.String(), "dev")
}

func TestListDevicesWithGui(t *testing.T) {
	fin := testDir.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	Run(&RunFlags{LogFile: "/dev/null", ShowDisks: true}, nil, true, buff, true)
}
