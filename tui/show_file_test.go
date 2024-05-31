package tui

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ulikunitz/xz"
)

func TestGetScannerForEmptyString(t *testing.T) {
	r := bytes.NewReader([]byte{})
	_, err := getScanner(r)
	assert.ErrorContains(t, err, "EOF")
}

func TestGetScannerForPlainString(t *testing.T) {
	r := bytes.NewReader([]byte("hello"))
	s, err := getScanner(r)
	assert.Nil(t, err)

	assert.Equal(t, true, s.Scan())
	assert.Equal(t, "hello", s.Text())
	assert.Equal(t, nil, s.Err())
}

func TestGetScannerForGzipped(t *testing.T) {
	b := bytes.NewBuffer([]byte{})
	w := gzip.NewWriter(b)

	_, err := w.Write([]byte("hello world"))
	assert.Nil(t, err)

	err = w.Close()
	assert.Nil(t, err)

	r := bytes.NewReader(b.Bytes())
	s, err := getScanner(r)
	assert.Nil(t, err)

	assert.Equal(t, true, s.Scan())
	assert.Equal(t, "hello world", s.Text())
	assert.Equal(t, nil, s.Err())
}

func TestGetScannerForBzipped(t *testing.T) {
	r := bytes.NewReader([]byte{
		// bzip2 header
		0x42, 0x5A, 0x68, 0x39,
		// bzip2 compressed data: "hello"
		0x31, 0x41, 0x59, 0x26,
		0x53, 0x59, 0xC1, 0xC0,
		0x80, 0xE2, 0x00, 0x00,
		0x01, 0x41, 0x00, 0x00,
		0x10, 0x02, 0x44, 0xA0,
		0x00, 0x30, 0xCD, 0x00,
		0xC3, 0x46, 0x29, 0x97,
		0x17, 0x72, 0x45, 0x38,
		0x50, 0x90, 0xC1, 0xC0,
		0x80, 0xE2,
	})
	s, err := getScanner(r)
	assert.Nil(t, err)

	assert.Equal(t, true, s.Scan())
	assert.Equal(t, "hello", s.Text())
	assert.Equal(t, nil, s.Err())
}

func TestGetScannerForXzipped(t *testing.T) {
	b := bytes.NewBuffer([]byte{})
	w, err := xz.NewWriter(b)
	assert.Nil(t, err)

	_, err = w.Write([]byte("hello world"))
	assert.Nil(t, err)

	err = w.Close()
	assert.Nil(t, err)

	r := bytes.NewReader(b.Bytes())
	s, err := getScanner(r)
	assert.Nil(t, err)

	assert.Equal(t, true, s.Scan())
	assert.Equal(t, "hello world", s.Text())
	assert.Equal(t, nil, s.Err())
}
