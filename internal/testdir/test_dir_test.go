package testdir

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateTestDir(t *testing.T) {
	cleanup := CreateTestDir()
	defer cleanup()

	// Verify directory structure exists
	info, err := os.Stat("test_dir")
	assert.NoError(t, err)
	assert.True(t, info.IsDir())

	info, err = os.Stat("test_dir/nested")
	assert.NoError(t, err)
	assert.True(t, info.IsDir())

	info, err = os.Stat("test_dir/nested/subnested")
	assert.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify file contents
	content, err := os.ReadFile("test_dir/nested/subnested/file")
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(content))

	content, err = os.ReadFile("test_dir/nested/file2")
	assert.NoError(t, err)
	assert.Equal(t, "go", string(content))
}

func TestCreateTestDirCleanup(t *testing.T) {
	cleanup := CreateTestDir()

	// Verify it exists
	_, err := os.Stat("test_dir")
	assert.NoError(t, err)

	// Run cleanup
	cleanup()

	// Verify it's gone
	_, err = os.Stat("test_dir")
	assert.True(t, os.IsNotExist(err))
}

func TestMockedPathChecker(t *testing.T) {
	info, err := MockedPathChecker("/any/path")
	assert.Nil(t, info)
	assert.Nil(t, err)

	info, err = MockedPathChecker("")
	assert.Nil(t, info)
	assert.Nil(t, err)

	info, err = MockedPathChecker("/another/different/path")
	assert.Nil(t, info)
	assert.Nil(t, err)
}
