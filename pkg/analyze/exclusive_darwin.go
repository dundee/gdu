//go:build darwin

package analyze

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// ATTR_CMNEXT_PRIVATESIZE from <sys/attr.h>: "the number of bytes that are not
// trapped inside a clone or snapshot, and which would be freed immediately if
// the file were deleted" (getattrlist(2)). golang.org/x/sys/unix has neither
// the constant nor a getattrlist wrapper.
const attrCmnextPrivateSize = 0x00000008

// privateSizeReply is the buffer getattrlist fills in for
// ATTR_CMN_RETURNED_ATTRS followed by ATTR_CMNEXT_PRIVATESIZE: the length of
// the reply, the attribute_set_t of what was returned, then the off_t. The
// off_t lands on an 8 byte boundary, so the Go layout matches the kernel's.
type privateSizeReply struct {
	_        uint32                          // length of the reply
	returned [unix.ATTR_BIT_MAP_COUNT]uint32 // common, volume, directory, file, fork
	private  int64
}

// errPrivateSizeNotSupported is returned when the private size of a file is not
// reported (HFS+, exFAT, network shares, ...).
var errPrivateSizeNotSupported = errors.New("private size not supported")

// libc holds getattrlist(2) and __error from libSystem. As in pkg/fs's
// dataless_darwin.go, they go through libSystem rather than a raw syscall
// because libSystem is the only ABI Apple guarantees (QA1118), and through
// purego because gdu builds with CGO_ENABLED=0. purego.SyscallN does not
// capture errno on darwin (it returned garbage for a missing file), so errno is
// read through __error, which returns the address of the calling thread's.
type libc struct {
	getattrlist func(path *byte, list *unix.Attrlist, reply *privateSizeReply, size uintptr, options uint32) int32
	errno       func() *int32
}

const libSystem = "/usr/lib/libSystem.B.dylib"

var loadLibc = sync.OnceValues(func() (*libc, error) {
	lib, err := purego.Dlopen(libSystem, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("dlopen %s: %w", libSystem, err)
	}
	c := &libc{}
	// Dlsym and RegisterFunc, not RegisterLibFunc, which panics on a missing
	// symbol.
	for name, fn := range map[string]any{"getattrlist": &c.getattrlist, "__error": &c.errno} {
		sym, err := purego.Dlsym(lib, name)
		if err != nil {
			return nil, fmt.Errorf("dlsym %s: %w", name, err)
		}
		purego.RegisterFunc(fn, sym)
	}
	return c, nil
})

// getattrlist issues getattrlist(2). It follows a final symlink, as FIEMAP's
// open does on Linux: with --follow-symlinks the analyzers pass a symlink's
// path together with its target's stat. It is a variable so that tests can feed
// replies the kernel would not hand out on demand.
var getattrlist = func(path string, list *unix.Attrlist, reply *privateSizeReply) error {
	c, err := loadLibc()
	if err != nil {
		return err
	}
	p, err := unix.BytePtrFromString(path)
	if err != nil {
		return err
	}
	// errno is per thread: read it on the thread that made the call.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if c.getattrlist(p, list, reply, unsafe.Sizeof(*reply), unix.FSOPT_ATTR_CMN_EXTENDED) != -1 {
		return nil
	}
	return syscall.Errno(*c.errno())
}

// privateSize returns the bytes of the file at path that no clone or snapshot
// shares, i.e. what deleting it would free.
func privateSize(path string) (int64, error) {
	list := unix.Attrlist{
		Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
		Commonattr:  unix.ATTR_CMN_RETURNED_ATTRS,
		Forkattr:    attrCmnextPrivateSize,
	}
	var reply privateSizeReply
	for {
		err := getattrlist(path, &list, &reply)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		break
	}
	if reply.returned[4]&attrCmnextPrivateSize == 0 {
		return 0, errPrivateSizeNotSupported
	}
	return reply.private, nil
}

// extentMeter measures exclusive usage. Unlike on Linux it remembers no
// filesystem as unsupported: a reply without the attribute is about that file,
// and asking costs a getattrlist, not an open.
type extentMeter struct{}

// measure returns the disk usage of a file as its private size, and the rest
// of its usage as shared. usage is the st_blocks figure; it is returned
// unchanged, with nothing shared, for anything that is not a regular file with
// allocated blocks or whose private size cannot be read.
//
// A file compressed by the filesystem (UF_COMPRESSED, decmpfs) keeps its data
// outside the data fork, and APFS reports its private size as 0 although no
// other file shares it (measured on macOS 27 with an uncloned compressed file
// whose clone reference count is 0, as is that of its clones), so it keeps its
// usage too: its clones and snapshots are counted as its own.
func (*extentMeter) measure(path string, info os.FileInfo, usage int64) (exclusive, shared int64) {
	if !info.Mode().IsRegular() || usage == 0 {
		// Nothing is allocated, so there is nothing that could be shared.
		return usage, 0
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Flags&unix.UF_COMPRESSED != 0 {
		return usage, 0
	}
	private, err := privateSize(path)
	if err != nil {
		return usage, 0
	}
	// Both figures count allocated blocks; keep exclusive and shared within
	// what st_blocks reports.
	private = min(private, usage)
	return private, usage - private
}
