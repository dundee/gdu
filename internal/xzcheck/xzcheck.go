// Package xzcheck checks what an xz file asks of the decoder before it is
// decompressed.
package xzcheck

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// MaxDict is the largest LZMA2 dictionary gdu lets an xz file ask for. The
// decoder allocates what each block header asks, up to 4 GiB, so a 128-byte
// file could make it allocate 1 GiB; xz -9 uses 64 MiB.
const MaxDict = 64 << 20

// ErrDictTooLarge is returned for a file asking for a larger dictionary.
var ErrDictTooLarge = errors.New("xz: the dictionary asked for is too large")

const (
	xzHeaderLen = 12
	xzFooterLen = 12
	// lzma2FilterID is the ID of the LZMA2 filter in a block header
	lzma2FilterID = 0x21
)

// CheckDicts reads the block headers of every stream of the xz file r,
// size bytes long, and fails when one asks for a dictionary larger than
// maxDict. The streams are found from the end: each footer locates its
// index, which gives the sizes of the blocks, and so where each block header
// starts. A file which is no valid xz fails too.
func CheckDicts(r io.ReaderAt, size, maxDict int64) error {
	end := size
	for end > 0 {
		// stream padding, in multiples of four zero bytes
		var word [4]byte
		if end < 4 {
			return errors.New("xz: truncated file")
		}
		if _, err := r.ReadAt(word[:], end-4); err != nil {
			return err
		}
		if word == [4]byte{} {
			end -= 4
			continue
		}
		start, err := checkXZStream(r, end, maxDict)
		if err != nil {
			return err
		}
		end = start
	}
	return nil
}

// checkXZStream checks the stream ending at end, and returns where it
// starts.
func checkXZStream(r io.ReaderAt, end, maxDict int64) (start int64, err error) {
	if end < xzHeaderLen+xzFooterLen {
		return 0, errors.New("xz: truncated stream")
	}
	footer := make([]byte, xzFooterLen)
	if _, err := r.ReadAt(footer, end-xzFooterLen); err != nil {
		return 0, err
	}
	if !bytes.Equal(footer[10:], []byte("YZ")) {
		return 0, errors.New("xz: no stream footer")
	}
	indexSize := (int64(binary.LittleEndian.Uint32(footer[4:8])) + 1) * 4
	indexStart := end - xzFooterLen - indexSize
	if indexStart < xzHeaderLen {
		return 0, errors.New("xz: index out of the file")
	}
	index := make([]byte, indexSize)
	if _, err := r.ReadAt(index, indexStart); err != nil {
		return 0, err
	}
	blocks, err := xzBlockSizes(index)
	if err != nil {
		return 0, err
	}

	var total int64
	for _, unpadded := range blocks {
		total += (unpadded + 3) &^ 3
	}
	start = indexStart - total - xzHeaderLen
	if start < 0 {
		return 0, errors.New("xz: blocks out of the file")
	}
	offset := start + xzHeaderLen
	for _, unpadded := range blocks {
		if err := checkXZBlockHeader(r, offset, maxDict); err != nil {
			return 0, err
		}
		offset += (unpadded + 3) &^ 3
	}
	return start, nil
}

// xzBlockSizes returns the unpadded sizes of the blocks the index lists.
func xzBlockSizes(index []byte) ([]int64, error) {
	if len(index) == 0 || index[0] != 0 {
		return nil, errors.New("xz: no index")
	}
	rest := index[1:]
	count, n := binary.Uvarint(rest)
	// a record takes two bytes at least
	if n <= 0 || count > uint64(len(rest))/2 {
		return nil, errors.New("xz: invalid index")
	}
	rest = rest[n:]
	sizes := make([]int64, 0, count)
	for range count {
		unpadded, n := binary.Uvarint(rest)
		if n <= 0 || unpadded > 1<<62 {
			return nil, errors.New("xz: invalid index record")
		}
		rest = rest[n:]
		if _, n = binary.Uvarint(rest); n <= 0 {
			return nil, errors.New("xz: invalid index record")
		}
		rest = rest[n:]
		sizes = append(sizes, int64(unpadded))
	}
	return sizes, nil
}

// checkXZBlockHeader checks the dictionary the LZMA2 filter of the block
// header at offset asks for.
func checkXZBlockHeader(r io.ReaderAt, offset, maxDict int64) error {
	var first [1]byte
	if _, err := r.ReadAt(first[:], offset); err != nil {
		return err
	}
	if first[0] == 0 {
		return errors.New("xz: no block header")
	}
	header := make([]byte, (int(first[0])+1)*4)
	if _, err := r.ReadAt(header, offset); err != nil {
		return err
	}
	flags := header[1]
	rest := header[2:]
	for _, present := range []bool{flags&0x40 != 0, flags&0x80 != 0} {
		// the compressed and uncompressed sizes
		if present {
			_, n := binary.Uvarint(rest)
			if n <= 0 {
				return errors.New("xz: invalid block header")
			}
			rest = rest[n:]
		}
	}
	for range int(flags&0x03) + 1 {
		id, n := binary.Uvarint(rest)
		if n <= 0 {
			return errors.New("xz: invalid filter")
		}
		rest = rest[n:]
		propsSize, n := binary.Uvarint(rest)
		if n <= 0 || propsSize > uint64(len(rest)-n) {
			return errors.New("xz: invalid filter")
		}
		props := rest[n : n+int(propsSize)]
		rest = rest[n+int(propsSize):]
		if id != lzma2FilterID {
			continue
		}
		if len(props) != 1 {
			return errors.New("xz: invalid LZMA2 filter")
		}
		dict, err := lzma.DecodeDictCap(props[0])
		if err != nil {
			return err
		}
		if dict > maxDict {
			return fmt.Errorf("%w: %d MiB", ErrDictTooLarge, dict>>20)
		}
	}
	return nil
}
