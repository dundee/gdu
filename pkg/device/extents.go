package device

import "math/bits"

// Extent is a contiguous byte range of a file's data on disk.
type Extent struct {
	// Physical is the byte offset on the underlying device, 0 when not allocated (see Inline, Delalloc).
	Physical uint64
	// Length is the logical length in bytes. For compressed extents this is the uncompressed
	// length, so it must not be used as the on-disk size.
	Length uint64
	// Encoded (compressed) extents are identified by their physical start only. The kernel reports
	// the uncompressed length for them, so Length says nothing about the space used on disk.
	Encoded bool
	// Inline extents live in the metadata tree, they have no physical address to deduplicate on.
	Inline bool
	// Delalloc extents are not written out yet, they have no physical address either.
	Delalloc bool
}

// extentBlockSize is the granularity physical ranges are tracked at, every btrfs sector is a multiple of it
const extentBlockSize = 4096

// extentChunkBlocks is how many blocks one bitmap chunk covers (1 GiB), chunks are only allocated when touched
const extentChunkBlocks = 1 << 18

// InlineKey identifies the content of a file stored inline in the metadata, which has no physical address.
// Snapshots keep inode numbers, so an unchanged file has the same key in every snapshot.
type InlineKey struct {
	Inode   uint64
	MtimeNs int64
	Size    int64
}

// ExtentSet remembers which on-disk data was already counted, so data shared between
// snapshots and reflinks is only attributed to the first file that references it.
type ExtentSet struct {
	chunks  map[uint64][]uint64
	encoded map[uint64]struct{}
	inline  map[InlineKey]struct{}
}

// NewExtentSet returns an empty set
func NewExtentSet() *ExtentSet {
	return &ExtentSet{
		chunks:  map[uint64][]uint64{},
		encoded: map[uint64]struct{}{},
		inline:  map[InlineKey]struct{}{},
	}
}

// Add records the extents of one file and returns how many of their bytes are seen for the
// first time and how many there are in total. Both are logical lengths: for compressed
// extents that is the uncompressed size, so callers should only use their ratio.
func (s *ExtentSet) Add(extents []Extent, inline InlineKey) (fresh, total uint64) {
	for _, extent := range extents {
		total += extent.Length
		switch {
		case extent.Delalloc:
			fresh += extent.Length
		case extent.Inline:
			if _, seen := s.inline[inline]; !seen {
				s.inline[inline] = struct{}{}
				fresh += extent.Length
			}
		case extent.Encoded:
			if _, seen := s.encoded[extent.Physical]; !seen {
				s.encoded[extent.Physical] = struct{}{}
				fresh += extent.Length
			}
		default:
			fresh += s.addRange(extent.Physical, extent.Length)
		}
	}
	return fresh, total
}

// addRange marks [start, start+length) as seen and returns the number of bytes that were not
func (s *ExtentSet) addRange(start, length uint64) uint64 {
	if length == 0 {
		return 0
	}
	first := start / extentBlockSize
	last := (start + length - 1) / extentBlockSize

	var fresh uint64
	for block := first; block <= last; {
		chunkIndex := block / extentChunkBlocks
		chunk, ok := s.chunks[chunkIndex]
		if !ok {
			chunk = make([]uint64, extentChunkBlocks/64)
			s.chunks[chunkIndex] = chunk
		}

		chunkEnd := min(last, (chunkIndex+1)*extentChunkBlocks-1)
		for block <= chunkEnd {
			offset := block % extentChunkBlocks
			word := offset / 64
			bit := offset % 64
			count := min(chunkEnd-block+1, 64-bit)

			mask := ^uint64(0) >> (64 - count) << bit
			fresh += uint64(bits.OnesCount64(mask &^ chunk[word]))
			chunk[word] |= mask
			block += count
		}
	}

	// partial first and last blocks are counted by their real byte span, not whole blocks
	blocksBytes := fresh * extentBlockSize
	return min(blocksBytes, length)
}
