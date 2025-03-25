package annex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnexedFileInfo(t *testing.T) {
	fi := &FileInfo{}
	fi = AnnexedFileInfo(fi, "SHA256E-s967858083--3e54803fded8dc3a9ea68b106f7b51e04e33c79b4a7b32a860f0b22d89af5c65.mp4")

	assert.Equal(t, int64(967858083), fi.Size())
}

func TestAnnexedFileInfoErr(t *testing.T) {
	fi := &FileInfo{}
	fi = AnnexedFileInfo(fi, "xxx")

	assert.Equal(t, int64(0), fi.Size())
}

func TestSizeFromKeyErr(t *testing.T) {
	_, err := SizeFromKey("xxx")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "key is is missing backend")

	_, err = SizeFromKey("SHA256E-sXXX--3e54803fded8dc3a9ea68b106f7b51e04e33c79b4a7b32a860f0b22d89af5c65.mp4")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse size")

	_, err = SizeFromKey("SHA256E-s--3e54803fded8dc3a9ea68b106f7b51e04e33c79b4a7b32a860f0b22d89af5c65.mp4")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse size")

	_, err = SizeFromKey("SHA256E-a-b-c--3e54803fded8dc3a9ea68b106f7b51e04e33c79b4a7b32a860f0b22d89af5c65.mp4")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "size not found in key")
}
