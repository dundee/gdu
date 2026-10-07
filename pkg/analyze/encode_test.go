package analyze

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestEncode(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name:  "test_dir",
			Size:  10,
			Usage: 18,
			Mtime: time.Date(2021, 8, 19, 0, 40, 0, 0, time.UTC),
		},
		ItemCount: 4,
		BasePath:  ".",
	}

	subdir := &Dir{
		File: &File{
			Name:   "nested",
			Size:   9,
			Usage:  14,
			Parent: dir,
		},
		ItemCount: 3,
	}
	file := &File{
		Name:   "file2",
		Size:   3,
		Usage:  4,
		Parent: subdir,
	}
	file2 := &File{
		Name:   "file",
		Size:   5,
		Usage:  6,
		Parent: subdir,
		Flag:   '@',
		Mtime:  time.Date(2021, 8, 19, 0, 40, 0, 0, time.UTC),
	}
	file3 := &File{
		Name: "file3",
		Mli:  1234,
		Flag: 'H',
	}
	dir.Files = fs.Files{subdir}
	subdir.Files = fs.Files{file, file2, file3}

	var buff bytes.Buffer
	err := dir.EncodeJSON(&buff, true, nil)

	assert.Nil(t, err)
	assert.Contains(t, buff.String(), `"name":"nested"`)
	assert.Contains(t, buff.String(), `"mtime":1629333600`)
	assert.Contains(t, buff.String(), `"ino":1234`)
	assert.Contains(t, buff.String(), `"hlnkc":true`)
	assert.Contains(t, buff.String(), `"asize":10,"dsize":18,"items":4`)
	assert.Contains(t, buff.String(), `"name":"nested","asize":9,"dsize":14,"items":3`)
}

func TestEncodeEmptyDir(t *testing.T) {
	dir := &Dir{File: &File{Name: "empty"}}

	var buff bytes.Buffer
	err := dir.EncodeJSON(&buff, false, nil)

	assert.NoError(t, err)
	assert.Equal(t, "[{\"name\":\"empty\",\"asize\":0,\"dsize\":0,\"items\":0}\n]", buff.String())
}

func TestEncodeSelectedAttributes(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name:  "test_dir",
			Size:  10,
			Usage: 18,
			Mtime: time.Date(2021, 8, 19, 0, 40, 0, 0, time.UTC),
		},
		BasePath: ".",
	}
	dir.AddFile(&File{
		Name:  "file",
		Size:  3,
		Usage: 4,
		Mtime: time.Date(2021, 8, 19, 0, 40, 0, 0, time.UTC),
		Flag:  '@',
	})

	var buff bytes.Buffer
	err := dir.EncodeJSON(&buff, true, fs.JSONAttributes{"asize": {}, "dsize": {}})

	assert.NoError(t, err)
	assert.Contains(t, buff.String(), `"name":"test_dir","asize":10,"dsize":18`)
	assert.Contains(t, buff.String(), `"name":"file","asize":3,"dsize":4`)
	assert.NotContains(t, buff.String(), `"mtime"`)
	assert.NotContains(t, buff.String(), `"notreg"`)
}

func TestEncodeSelectedAttributesWithIno(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name: "test_dir",
		},
		BasePath: ".",
	}
	dir.AddFile(&File{
		Name: "link",
		Size: 10,
		Mli:  5678,
		Flag: 'H',
	})

	var buff bytes.Buffer
	err := dir.EncodeJSON(&buff, true, fs.JSONAttributes{"name": {}, "ino": {}})

	assert.NoError(t, err)
	assert.Contains(t, buff.String(), `"name":"link"`)
	assert.Contains(t, buff.String(), `"ino":5678,"hlnkc":true`)
	assert.NotContains(t, buff.String(), `"asize"`)
}

func TestEncodeSelectedAttributesWithoutIno(t *testing.T) {
	dir := &Dir{
		File: &File{
			Name: "test_dir",
		},
		BasePath: ".",
	}
	dir.AddFile(&File{
		Name: "link",
		Size: 10,
		Mli:  5678,
		Flag: 'H',
	})

	var buff bytes.Buffer
	err := dir.EncodeJSON(&buff, true, fs.JSONAttributes{"name": {}, "asize": {}})

	assert.NoError(t, err)
	assert.Contains(t, buff.String(), `"name":"link","asize":10`)
	assert.NotContains(t, buff.String(), `"ino"`)
	assert.NotContains(t, buff.String(), `"hlnkc"`)
}

func TestEncodeReadError(t *testing.T) {
	dir := &Dir{File: &File{Name: "test_dir"}, BasePath: "."}
	dir.AddFile(&Dir{File: &File{Name: "unreadable", Flag: '!'}})
	dir.AddFile(&Dir{File: &File{Name: "partial", Flag: '.'}})

	var buff bytes.Buffer
	assert.NoError(t, dir.EncodeJSON(&buff, true, fs.JSONAttributes{"name": {}}))
	assert.Contains(t, buff.String(), `[{"name":"unreadable","read_error":true}`)
	assert.Contains(t, buff.String(), `[{"name":"partial"}`)
	assert.Contains(t, buff.String(), `[{"name":"test_dir"}`)
}

func TestExportMarksAnUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	require.NoError(t, os.Mkdir(locked, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "file"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	a := CreateAnalyzer()
	root := a.AnalyzeDir(dir, func(_, _ string) bool { return false }, func(_ string) bool { return false })
	a.GetDone().Wait()
	root.UpdateStats(make(fs.HardLinkedItems))

	var buff bytes.Buffer
	require.NoError(t, root.EncodeJSON(&buff, true, nil))
	assert.Contains(t, buff.String(), `{"name":"locked"`)
	assert.Contains(t, buff.String(), `"read_error":true`)
}
