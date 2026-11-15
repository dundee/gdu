//go:build windows

package remove

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

func TestTrashCommandNotSupported(t *testing.T) {
	dir := &analyze.Dir{File: &analyze.File{Name: "test_dir"}}
	file := &analyze.File{Name: "file", Parent: dir}

	err := TrashCommand("rm -rf")(dir, file)
	assert.Error(t, err)
}
