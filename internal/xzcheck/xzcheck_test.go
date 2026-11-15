package xzcheck

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

func compress(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w, err := xz.WriterConfig{DictCap: 1 << 20}.NewWriter(&out)
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return out.Bytes()
}

// WithDict returns the xz file data, of one block, with its block header
// asking for a dictionary of dict bytes: a valid file, which decompresses.
func WithDict(t *testing.T, data []byte, dict int64) []byte {
	t.Helper()
	data = bytes.Clone(data)
	header := data[xzHeaderLen : xzHeaderLen+(int(data[xzHeaderLen])+1)*4]
	i := bytes.IndexByte(header[2:], lzma2FilterID)
	require.GreaterOrEqual(t, i, 0)
	header[2+i+2] = lzma.EncodeDictCap(dict)
	binary.LittleEndian.PutUint32(header[len(header)-4:], crc32.ChecksumIEEE(header[:len(header)-4]))
	return data
}

func TestCheckDicts(t *testing.T) {
	small := compress(t, []byte("hello"))
	huge := WithDict(t, small, 1<<30)
	// a file of several streams, with padding between them
	streams := append(append(bytes.Clone(small), 0, 0, 0, 0), small...)

	for name, c := range map[string]struct {
		data []byte
		want error
	}{
		"small":       {small, nil},
		"streams":     {streams, nil},
		"huge":        {huge, ErrDictTooLarge},
		"huge second": {append(append(bytes.Clone(small), 0, 0, 0, 0), huge...), ErrDictTooLarge},
		"not xz":      {[]byte("hello, this is no xz file at all"), nil},
		"empty":       {nil, nil},
		"cut":         {small[:len(small)-3], nil},
	} {
		err := CheckDicts(bytes.NewReader(c.data), int64(len(c.data)), MaxDict)
		switch {
		case c.want != nil:
			assert.ErrorIs(t, err, c.want, name)
		case name == "small" || name == "streams" || name == "empty":
			assert.NoError(t, err, name)
		default:
			assert.Error(t, err, name)
		}
	}
}

// The patched file is still valid xz, so the check is what keeps the
// decoder from allocating the dictionary.
func TestWithDictDecompresses(t *testing.T) {
	data := WithDict(t, compress(t, []byte("hello")), 2<<20)
	r, err := xz.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	var out bytes.Buffer
	_, err = out.ReadFrom(r)
	require.NoError(t, err)
	assert.Equal(t, "hello", out.String())
}
