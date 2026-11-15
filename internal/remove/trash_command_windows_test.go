//go:build windows

package remove

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v6/pkg/analyze"
)

func TestTrashCommandNotSupported(t *testing.T) {
	dir := &analyze.Dir{File: &analyze.File{Name: "test_dir"}}
	file := &analyze.File{Name: "file", Parent: dir}

	_, err := TrashCommand("rm -rf")(file)
	assert.Error(t, err)
}
