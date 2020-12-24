package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessDir(t *testing.T) {
	os.MkdirAll("test_dir/nested/subnested", os.ModePerm)
	ioutil.WriteFile("test_dir/nested/subnested/file", []byte("hello"), 0644)
	ioutil.WriteFile("test_dir/nested/file2", []byte("go"), 0644)

	statusChannel := make(chan CurrentProgress)

	dir := processDir("test_dir", statusChannel)

	// test dir info
	assert.Equal(t, "test_dir", dir.name)
	assert.Equal(t, int64(7), dir.size)
	assert.Equal(t, 5, dir.itemCount)
	assert.True(t, dir.isDir)

	// test dir tree
	assert.Equal(t, "nested", dir.files[0].name)
	assert.Equal(t, "subnested", dir.files[0].files[0].name)

	// test file
	assert.Equal(t, "file2", dir.files[0].files[1].name)
	assert.Equal(t, int64(2), dir.files[0].files[1].size)

	assert.Equal(t, "file", dir.files[0].files[0].files[0].name)
	assert.Equal(t, int64(5), dir.files[0].files[0].files[0].size)

	// test parent link
	assert.Equal(t, "test_dir", dir.files[0].files[0].files[0].parent.parent.parent.name)
}
