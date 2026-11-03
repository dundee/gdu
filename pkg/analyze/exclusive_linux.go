//go:build linux

package analyze

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// FIEMAP ioctl, see Documentation/filesystems/fiemap.rst in the Linux kernel.
// golang.org/x/sys/unix has no binding for it. Extent flags that need no
// special handling (UNKNOWN, DELALLOC, ENCODED, DATA_INLINE, UNWRITTEN) are
// counted like any other extent, see extentBytes.
const (
	fiemapExtentLast        = 0x00000001
	fiemapExtentShared      = 0x00002000
	fiemapExtentsPerRequest = 256
	fiemapMaxOffset         = ^uint64(0)
)

type fiemapExtent struct {
	logical    uint64
	physical   uint64
	length     uint64
	reserved64 [2]uint64
	flags      uint32
	reserved   [3]uint32
}

type fiemapRequest struct {
	start       uint64
	length      uint64
	flags       uint32
	mappedCount uint32
	extentCount uint32
	reserved    uint32
	extents     [fiemapExtentsPerRequest]fiemapExtent
}

// ioctlFiemap is FS_IOC_FIEMAP, _IOWR('f', 11, struct fiemap) with a 32 byte
// header. The value is the same under the generic ioctl encoding and the
// mips/powerpc/sparc one, as the read and write direction bits add up to the
// same top two bits.
const ioctlFiemap = 0xC020660B

// fiemapIoctl issues FS_IOC_FIEMAP. It is a variable so that tests can feed
// extent maps the kernel would not hand out on demand.
var fiemapIoctl = func(fd int, req *fiemapRequest) syscall.Errno {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), ioctlFiemap, uintptr(unsafe.Pointer(req)))
	return errno
}

var fiemapPool = sync.Pool{New: func() any { return new(fiemapRequest) }}

// errFiemapNotSupported is returned when exclusive bytes cannot be measured
// and the caller should fall back to st_blocks.
var errFiemapNotSupported = errors.New("fiemap not supported")

// extentBytes walks the extent map of the regular file at path with
// FS_IOC_FIEMAP, asking the kernel for at most perRequest extents per call (at
// most fiemapExtentsPerRequest). It returns the bytes not shared with any other
// file, i.e. what deleting it would free, and the bytes in extents flagged
// FIEMAP_EXTENT_SHARED, which deleting the file alone does not free.
//
// Holes are not reported by the kernel and therefore count as nothing.
// Preallocated (unwritten), delayed-allocation, not-yet-located and inline
// extents are real space owned by the file, so they are counted unless flagged
// as shared. Lengths are logical: a compressed extent is counted at its
// uncompressed length.
func extentBytes(path string, perRequest uint32) (exclusive, shared int64, err error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return 0, 0, err
	}
	defer unix.Close(fd) //nolint:errcheck // Read-only descriptor, nothing to flush.

	req := fiemapPool.Get().(*fiemapRequest)
	defer fiemapPool.Put(req)

	var exclusiveTotal, sharedTotal uint64
	var start uint64
	for {
		*req = fiemapRequest{} // the kernel only fills in what it maps
		req.start = start
		req.length = fiemapMaxOffset - start
		req.extentCount = perRequest

		errno := fiemapIoctl(fd, req)
		if errno != 0 {
			if errno == syscall.EINTR {
				continue
			}
			return 0, 0, errno
		}

		n := req.mappedCount
		if n > perRequest {
			return 0, 0, errFiemapNotSupported
		}
		if n == 0 {
			return int64(exclusiveTotal), int64(sharedTotal), nil
		}
		for i := range n {
			e := &req.extents[i]
			if e.flags&fiemapExtentShared == 0 {
				exclusiveTotal += e.length
			} else {
				sharedTotal += e.length
			}
			if e.flags&fiemapExtentLast != 0 {
				return int64(exclusiveTotal), int64(sharedTotal), nil
			}
		}

		last := &req.extents[n-1]
		next := last.logical + last.length
		if next <= start || next < last.logical {
			// No forward progress or overflow: stop rather than loop forever.
			return 0, 0, errFiemapNotSupported
		}
		start = next
	}
}

// measureExclusiveUsage returns the disk usage of a file as its exclusive
// bytes, and the bytes in its shared extents. usage is the st_blocks figure;
// it is returned unchanged, with nothing shared, for anything that is not a
// regular file with allocated blocks or whose extent map cannot be read.
func measureExclusiveUsage(path string, info os.FileInfo, usage int64) (exclusive, shared int64) {
	if !info.Mode().IsRegular() || usage == 0 {
		// Nothing is allocated, so there is nothing that could be shared.
		return usage, 0
	}
	exclusive, shared, err := extentBytes(path, fiemapExtentsPerRequest)
	if err != nil {
		return usage, 0
	}
	return exclusive, shared
}
