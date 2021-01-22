package cmd

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/analyze"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {

	buff := bytes.NewBuffer(make([]byte, 10))

	scan(&scanFlags{showVersion: true}, []string{}, false, buff)

	assert.Contains(t, buff.String(), "Version:\t development")
}

func TestAnalyzePath(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	scan(&scanFlags{logFile: "/dev/null"}, []string{"test_dir"}, false, buff)

	assert.Contains(t, buff.String(), "nested")
}

func TestListDevices(t *testing.T) {
	fin := analyze.CreateTestDir()
	defer fin()

	buff := bytes.NewBuffer(make([]byte, 10))

	scan(&scanFlags{logFile: "/dev/null", showDisks: true}, nil, false, buff)

	assert.Contains(t, buff.String(), "dev")
}
