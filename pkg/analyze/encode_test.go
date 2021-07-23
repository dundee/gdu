package analyze

import (
	"bytes"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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
	}
	dir.Files = Files{subdir}
	subdir.Files = Files{file, file2}

	var buff bytes.Buffer
	err := dir.EncodeJSON(&buff, true)

	assert.Nil(t, err)
	assert.Contains(t, buff.String(), `"name":"nested"`)
}
