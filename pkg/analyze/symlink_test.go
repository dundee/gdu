package analyze

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
)

func TestFollowSymlinkErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Mkdir("test_dir/empty", 0o644)
	assert.Nil(t, err)

	err = os.Symlink(
		".git/annex/objects/qx/qX/SHA256E-s967858083--"+
			"3e54803fded8dc3a9ea68b106f7b51e04e33c79b4a7b32a860f0b22d89af5c65.mp4/SHA256E-s967858083--"+
			"3e54803fded8dc3a9ea68b106f7b51e04e33c79b4a7b32a860f0b22d89af5c65.mp4",
		"test_dir/nested/file3")
	assert.Nil(t, err)

	err = os.Symlink(
		"test_dir/nested",
		"test_dir/some_dir")
	assert.Nil(t, err)

	_, err = followSymlink("xxx", false)
	assert.ErrorContains(t, err, "no such file or directory")

	_, err = followSymlink("test_dir/nested/file3", false)
	assert.ErrorContains(t, err, "no such file or directory")

	_, err = followSymlink("test_dir/nested/file3", true)
	assert.NoError(t, err)

	res, err := followSymlink("some_dir", true)
	assert.Equal(t, nil, res)
	assert.NoError(t, err)
}
