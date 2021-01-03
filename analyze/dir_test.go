package analyze

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessDir(t *testing.T) {
	fin := CreateTestDir()
	defer fin()

	dir := ProcessDir("test_dir", &CurrentProgress{Mutex: &sync.Mutex{}}, func(_ string) bool { return false })

	// test dir info
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, int64(7), dir.Size)
	assert.Equal(t, 5, dir.ItemCount)
	assert.True(t, dir.IsDir)

	// test dir tree
	assert.Equal(t, "nested", dir.Files[0].Name)
	assert.Equal(t, "subnested", dir.Files[0].Files[1].Name)

	// test file
	assert.Equal(t, "file2", dir.Files[0].Files[0].Name)
	assert.Equal(t, int64(2), dir.Files[0].Files[0].Size)

	assert.Equal(t, "file", dir.Files[0].Files[1].Files[0].Name)
	assert.Equal(t, int64(5), dir.Files[0].Files[1].Files[0].Size)

	// test parent link
	assert.Equal(t, "test_dir", dir.Files[0].Files[1].Files[0].Parent.Parent.Parent.Name)
}

func TestIgnoreDir(t *testing.T) {
	fin := CreateTestDir()
	defer fin()

	dir := ProcessDir("test_dir", &CurrentProgress{Mutex: &sync.Mutex{}}, func(_ string) bool { return true })

	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, 1, dir.ItemCount)
}
