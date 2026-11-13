package path

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortenPath(t *testing.T) {
	assert.Equal(t, "/root", ShortenPath("/root", 10))
	assert.Equal(t, "/home/.../foo", ShortenPath("/home/dundee/foo", 10))
	assert.Equal(t, "/home/dundee/foo", ShortenPath("/home/dundee/foo", 50))
	assert.Equal(t, "/home/dundee/.../bar.txt", ShortenPath("/home/dundee/foo/bar.txt", 20))
	assert.Equal(t, "/home/.../bar.txt", ShortenPath("/home/dundee/foo/bar.txt", 15))
}
