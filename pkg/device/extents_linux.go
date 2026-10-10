package device

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctl FS_IOC_FIEMAP, _IOWR('f', 11, struct fiemap)
const fsIocFiemap = 0xC020660B

const (
	fiemapExtentLast       = 0x1
	fiemapExtentDelalloc   = 0x4
	fiemapExtentEncoded    = 0x8
	fiemapExtentDataInline = 0x200
	fiemapExtentUnwritten  = 0x800

	fiemapHeaderSize = 32
	fiemapExtentSize = 56
	// extents fetched per ioctl call
	fiemapBatch = 256
)

// ReadExtents returns the extents backing a regular file using FS_IOC_FIEMAP.
// Holes and preallocated (unwritten) ranges are left out as they take no data space.
func ReadExtents(path string) ([]Extent, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// uint64 backed so the header and extents are 8 byte aligned
	buffer := make([]uint64, (fiemapHeaderSize+fiemapBatch*fiemapExtentSize)/8)
	extents := []Extent{}
	var start uint64

	for {
		// struct fiemap: start, length, then u32s: flags, mapped_extents, extent_count, reserved
		buffer[0] = start
		buffer[1] = ^uint64(0) - start
		buffer[2] = 0           // flags (low) and mapped_extents (high), the kernel fills the latter
		buffer[3] = fiemapBatch // extent_count (low) and reserved (high)
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, file.Fd(), fsIocFiemap, uintptr(unsafe.Pointer(&buffer[0])))
		if errno != 0 {
			return nil, fmt.Errorf("fiemap %s: %w", path, errno)
		}

		mapped := int(buffer[2] >> 32)
		if mapped == 0 {
			return extents, nil
		}

		last := false
		for i := 0; i < mapped; i++ {
			// struct fiemap_extent: logical, physical, length, reserved[2], flags, reserved[3]
			base := (fiemapHeaderSize + i*fiemapExtentSize) / 8
			logical, physical, length := buffer[base], buffer[base+1], buffer[base+2]
			flags := uint32(buffer[base+5] & 0xffffffff)

			start = logical + length
			last = flags&fiemapExtentLast != 0
			if flags&fiemapExtentUnwritten != 0 {
				continue
			}
			extents = append(extents, Extent{
				Physical: physical,
				Length:   length,
				Encoded:  flags&fiemapExtentEncoded != 0,
				Inline:   flags&fiemapExtentDataInline != 0,
				Delalloc: flags&fiemapExtentDelalloc != 0,
			})
		}
		if last {
			return extents, nil
		}
	}
}
