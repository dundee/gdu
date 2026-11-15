package analyze

import (
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestStoredRecordRoundTrip(t *testing.T) {
	record := &storedRecord{
		Format:    storedFormat,
		Type:      "ZipDirectory",
		Flag:      '!',
		Size:      1 << 40,
		Usage:     4096,
		OwnUsage:  4096,
		ItemCount: 4,
		Dev:       1<<40 + 5,
		Mtime:     time.Unix(1_700_000_000, 123_456_789),
		Entries: []storedEntry{
			{Name: "sub", Dir: true},
			{
				Name: "файл ✓", Flag: 'H', Size: 12345, Usage: 16384, SharedUsage: 8192,
				Mtime: time.Unix(1_600_000_000, 1), Mli: 987654321,
			},
			{Name: "link", Flag: '@', Symlink: "../target"},
			{Name: "at-the-epoch", Mtime: time.Unix(0, 0)},
			{Name: "before-the-epoch", Mtime: time.Unix(-86400, 500)},
			{Name: "never-set"},
			{Name: "negative", Size: -1, Usage: -4096, Flag: -1},
		},
	}

	var loaded storedRecord
	require.NoError(t, loaded.unmarshal(record.marshal()))

	assert.Equal(t, record, &loaded)
	assert.True(t, loaded.Entries[5].Mtime.IsZero())
	assert.False(t, loaded.Entries[3].Mtime.IsZero())
}

func TestStoredRecordOfEmptyDirectory(t *testing.T) {
	record := &storedRecord{Format: storedFormat, ItemCount: 1}

	var loaded storedRecord
	require.NoError(t, loaded.unmarshal(record.marshal()))

	assert.Equal(t, storedFormat, loaded.Format)
	assert.Equal(t, int64(1), loaded.ItemCount)
	assert.Empty(t, loaded.Entries)
	assert.True(t, loaded.Mtime.IsZero())
}

// A later gdu may add fields; this one must still read the fields it knows.
func TestStoredRecordSkipsUnknownFields(t *testing.T) {
	entry := (&storedEntry{Name: "f", Size: 7}).marshal(nil)
	entry = protowire.AppendTag(entry, 99, protowire.Fixed64Type)
	entry = protowire.AppendFixed64(entry, 1)

	value := (&storedRecord{Format: storedFormat, Size: 7}).marshal()
	value = protowire.AppendTag(value, 98, protowire.BytesType)
	value = protowire.AppendString(value, "future")
	value = protowire.AppendTag(value, protowire.Number(recordEntries), protowire.BytesType)
	value = protowire.AppendBytes(value, entry)

	var loaded storedRecord
	require.NoError(t, loaded.unmarshal(value))

	assert.Equal(t, int64(7), loaded.Size)
	assert.Equal(t, []storedEntry{{Name: "f", Size: 7}}, loaded.Entries)
}

func TestStoredRecordMalformed(t *testing.T) {
	value := (&storedRecord{
		Format:  storedFormat,
		Entries: []storedEntry{{Name: "a-file-name", Size: 100}},
	}).marshal()

	// cut anywhere inside the entry, which follows the format field
	formatField := len(appendVarint(nil, recordFormat, storedFormat))
	for len(value) > formatField+1 {
		value = value[:len(value)-1]
		var loaded storedRecord
		assert.Error(t, loaded.unmarshal(value), "truncated to %d bytes", len(value))
	}
}

func TestStoredRecordOfOtherFormatIsRefused(t *testing.T) {
	storage := NewStorage(t.TempDir())
	closeFn := storage.Open()
	defer closeFn()
	require.NoError(t, storage.storeRecord("/dir", &storedRecord{}))
	value := (&storedRecord{Format: storedFormat - 1}).marshal()
	require.NoError(t, storage.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("/dir"), value)
	}))

	_, err := storage.loadRecord("/dir")

	assert.ErrorIs(t, err, errStoredFormat)
}
