package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtentSetCountsSharedRangesOnce(t *testing.T) {
	set := NewExtentSet()
	const mib = 1 << 20

	// first file references a 20 MiB extent
	fresh, total := set.Add([]Extent{{Physical: 100 * mib, Length: 20 * mib}}, InlineKey{})
	assert.Equal(t, uint64(20*mib), fresh)
	assert.Equal(t, uint64(20*mib), total)

	// a snapshot of it after 5 MiB were rewritten elsewhere: 3 MiB + 12 MiB of the old extent stay shared
	fresh, total = set.Add([]Extent{
		{Physical: 100 * mib, Length: 3 * mib},
		{Physical: 500 * mib, Length: 5 * mib},
		{Physical: 108 * mib, Length: 12 * mib},
	}, InlineKey{})
	assert.Equal(t, uint64(5*mib), fresh)
	assert.Equal(t, uint64(20*mib), total)

	// 8 MiB to 11 MiB of the old extent is still unseen
	fresh, _ = set.Add([]Extent{{Physical: 100 * mib, Length: 20 * mib}}, InlineKey{})
	assert.Equal(t, uint64(0), fresh)
}

func TestExtentSetPartialOverlapAcrossChunks(t *testing.T) {
	set := NewExtentSet()
	const gib = 1 << 30

	fresh, _ := set.Add([]Extent{{Physical: gib - 8192, Length: 16384}}, InlineKey{})
	assert.Equal(t, uint64(16384), fresh)

	fresh, _ = set.Add([]Extent{{Physical: gib - 4096, Length: 16384}}, InlineKey{})
	assert.Equal(t, uint64(4096), fresh, "only the part past the first extent is new")
}

func TestExtentSetEncodedByStartAndInlineByKey(t *testing.T) {
	set := NewExtentSet()

	extents := []Extent{{Physical: 4096, Length: 131072, Encoded: true}}
	fresh, _ := set.Add(extents, InlineKey{})
	assert.Equal(t, uint64(131072), fresh)
	fresh, _ = set.Add(extents, InlineKey{})
	assert.Equal(t, uint64(0), fresh, "same compressed extent counted once")

	inline := []Extent{{Length: 4096, Inline: true}}
	fresh, _ = set.Add(inline, InlineKey{Inode: 7, MtimeNs: 1, Size: 5})
	assert.Equal(t, uint64(4096), fresh)
	fresh, _ = set.Add(inline, InlineKey{Inode: 7, MtimeNs: 1, Size: 5})
	assert.Equal(t, uint64(0), fresh, "unchanged inline file in a snapshot")
	fresh, _ = set.Add(inline, InlineKey{Inode: 7, MtimeNs: 2, Size: 5})
	assert.Equal(t, uint64(4096), fresh, "modified inline file")
}
