package report

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestReadAnalysis(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`
		[1,2,{"progname":"gdu","progver":"development","timestamp":1626806293},
		[{"name":"/home/xxx","asize":47000000,"dsize":64000000,"items":6,"mtime":1629333600},
		{"name":"gdu.json","asize":33805233,"dsize":33808384},
		{"name":"sock","notreg":true},
		[{"name":"app","asize":10022,"dsize":20480,"items":4},
		{"name":"app.go","asize":4638,"dsize":8192},
		{"name":"app_linux_test.go","asize":1410,"dsize":4096},
		{"name":"app_linux_test2.go","ino":1234,"hlnkc":true,"asize":1410,"dsize":4096},
		{"name":"app_test.go","asize":4974,"dsize":8192}],
		{"name":"main.go","asize":3205,"dsize":4096,"mtime":1629333600}]]
	`))

	dir, err := ReadAnalysis(buff)

	assert.Nil(t, err)
	assert.Equal(t, "xxx", dir.GetName())
	assert.Equal(t, "/home/xxx", dir.GetPath())
	assert.Equal(t, int64(47000000), dir.GetSize())
	assert.Equal(t, int64(64000000), dir.GetUsage())
	assert.Equal(t, int64(6), dir.GetItemCount())
	assert.Equal(t, int64(10022), dir.Files[2].GetSize())
	assert.Equal(t, int64(20480), dir.Files[2].GetUsage())
	assert.Equal(t, int64(4), dir.Files[2].GetItemCount())
	alt2 := dir.Files[2].(*analyze.Dir).Files[2].(*analyze.File)
	assert.Equal(t, "app_linux_test2.go", alt2.Name)
	assert.Equal(t, uint64(1234), alt2.Mli)
	assert.Equal(t, 'H', alt2.Flag)
}

func TestReadAnalysisPreservesTruncatedDirectoryStats(t *testing.T) {
	input := bytes.NewBufferString(`
		[1,2,{"progname":"gdu","progver":"development","timestamp":0},
		[{"name":"/root","asize":4096,"dsize":4096,"items":3},
		[{"name":"summary","asize":4096,"dsize":4096,"items":2}]]]
	`)

	dir, err := ReadAnalysis(input)
	assert.NoError(t, err)
	dir.UpdateStats(make(fs.HardLinkedItems, 10))

	assert.Equal(t, int64(4096), dir.GetSize())
	assert.Equal(t, int64(4096), dir.GetUsage())
	assert.Equal(t, int64(3), dir.GetItemCount())
	summary := dir.Files[0].(*analyze.Dir)
	assert.Equal(t, int64(4096), summary.GetSize())
	assert.Equal(t, int64(4096), summary.GetUsage())
	assert.Equal(t, int64(2), summary.GetItemCount())
	dir.UpdateStatsWithFileFiltering(make(fs.HardLinkedItems, 10))
	assert.Equal(t, int64(4096), dir.GetSize())
	assert.Equal(t, int64(4096), dir.GetUsage())
	assert.Equal(t, int64(3), dir.GetItemCount())
}

func TestReadAnalysisRecomputesEmptyDirStats(t *testing.T) {
	input := bytes.NewBufferString(`
		[1,2,{"progname":"gdu","progver":"development","timestamp":0},
		[{"name":"/empty","asize":512,"dsize":0,"items":1}]]
	`)

	dir, err := ReadAnalysis(input)
	assert.NoError(t, err)
	dir.UpdateStats(make(fs.HardLinkedItems, 10))

	assert.Equal(t, int64(512), dir.GetSize())
	assert.Equal(t, int64(0), dir.GetUsage())
	assert.Equal(t, int64(1), dir.GetItemCount())
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

	assert.Equal(t, "top level array must have at least 4 items", err.Error())
}

func TestReadAnalysisWithWrongContent(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,4]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "array of maps not found in the top level array on 4th position", err.Error())
}

func TestReadAnalysisWithEmptyContent(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,[]]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "directory array is empty", err.Error())
}

func TestReadAnalysisWithEmptyDirContent(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,[{}]]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "directory name is not a string", err.Error())
}

func TestReadAnalysisWithWrongDirItem(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,[1, 2, 3]]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "directory item is not a map", err.Error())
}

func TestReadAnalysisWithWrongName(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,[{"name":"/"},{"name":42}]]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "file name is not a string", err.Error())
}

func TestReadAnalysisWithWrongSubdirItem(t *testing.T) {
	buff := bytes.NewBuffer([]byte(`[1,2,3,[{"name":"xxx"}, [1,2,3]]]`))

	_, err := ReadAnalysis(buff)

	assert.Equal(t, "directory item is not a map", err.Error())
}

type BrokenInput struct{}

func (i *BrokenInput) Read(p []byte) (n int, err error) {
	return 0, errors.New("IO error")
}
