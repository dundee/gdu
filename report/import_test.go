package report

import (
	"bytes"
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestReadAnalysis(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`
		[1,2,{"progname":"gdu","progver":"development","timestamp":1626806293},
		[{"name":"/home/xxx"},
		{"name":"gdu.json","asize":33805233,"dsize":33808384},
		{"name":"sock","notreg":true},
		[{"name":"app"},
		{"name":"app.go","asize":4638,"dsize":8192},
		{"name":"app_linux_test.go","asize":1410,"dsize":4096},
		{"name":"app_test.go","asize":4974,"dsize":8192}],
		{"name":"main.go","asize":3205,"dsize":4096}]]
	`))

	dir, err := ReadAnalysis(buff)

	assert.Nil(t, err)
	assert.Equal(t, "xxx", dir.GetName())
	assert.Equal(t, "/home/xxx", dir.GetPath())
}

func TestReadAnalysisWithEmptyInput(t *testing.T) {
	buff := bytes.NewBuffer([]byte(``))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "unexpected end of JSON input", err.Error())
}

func TestReadAnalysisWithEmptyDict(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`{}`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "JSON file does not contain top level array", err.Error())
}

func TestReadFromBrokenInput(t *testing.T) {
	_, err := ReadAnalysis(&BrokenInput{})

	assert.Equal(t, "IO error", err.Error())
}

func TestReadAnalysisWithEmptyArray(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "Top level array must have at least 4 items", err.Error())
}

func TestReadAnalysisWithWrongContent(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,4]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "Array of maps not found in the top level array on 4th position", err.Error())
}

func TestReadAnalysisWithEmptyDirContent(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,[{}]]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "Directory name is not a string", err.Error())
}

func TestReadAnalysisWithWrongDirItem(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,[1, 2, 3]]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "Directory item is not a map", err.Error())
}

func TestReadAnalysisWithWrongSubdirItem(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,[{"name":"xxx"}, [1,2,3]]]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "Directory item is not a map", err.Error())
}

type BrokenInput struct{}

func (i *BrokenInput) Read(p []byte) (n int, err error) {
	return 0, errors.New("IO error")
}
