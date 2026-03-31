package common_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestSetIgnoreFromEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ignore")
	err := os.WriteFile(path, []byte(""), 0o600)
	assert.Nil(t, err)

	ui := &common.UI{}
	err = ui.SetIgnoreFromFile(path)
	assert.Nil(t, err)

	shouldIgnore := ui.CreateIgnoreFunc()
	assert.False(t, shouldIgnore("anything", "/anything"))
}

func TestSetIgnoreFromFileWithBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ignore")
	content := "/valid\n/other\n"
	err := os.WriteFile(path, []byte(content), 0o600)
	assert.Nil(t, err)

	ui := &common.UI{}
	err = ui.SetIgnoreFromFile(path)
	assert.Nil(t, err)

	shouldIgnore := ui.CreateIgnoreFunc()
	assert.True(t, shouldIgnore("valid", "/valid"))
	assert.True(t, shouldIgnore("other", "/other"))
	assert.False(t, shouldIgnore("xxx", "/xxx"))
}
