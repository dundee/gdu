//go:build darwin

package fs

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// Constants from the macOS SDK, sys/resource.h. Documented in setiopolicy_np(3).
const (
	iopolTypeVFSMaterializeDatalessFiles = 3 // IOPOL_TYPE_VFS_MATERIALIZE_DATALESS_FILES
	iopolScopeProcess                    = 0 // IOPOL_SCOPE_PROCESS
	iopolMaterializeDatalessFilesOff     = 1 // IOPOL_MATERIALIZE_DATALESS_FILES_OFF
)

const libSystem = "/usr/lib/libSystem.B.dylib"

// PreventDatalessMaterialization stops the kernel from materializing dataless
// files while this process walks the filesystem.
//
// On volumes backed by a File Provider extension (iCloud Drive, Google Drive,
// OneDrive, Dropbox) files can be "dataless": the metadata is local but the
// contents are not. Traversing such a directory makes the kernel ask the
// provider to materialize it. That is a synchronous round trip to the provider
// daemon and often a real download, so scanning is orders of magnitude slower
// and pulls down data the user never asked for.
//
// /usr/bin/du opts out for the same reason, by setting the
// vfs.nspace.prevent_materialization sysctl (rdar://44903941):
// https://github.com/apple-oss-distributions/file_cmds/blob/659a8a301e2acf0343f8b8673a154a2ca4d07084/du/du.c#L323
//
// We use setiopolicy_np instead. It sets the same per-process kernel state, and
// it is the API Apple documents for this in TN3150:
// https://developer.apple.com/documentation/technotes/tn3150-getting-ready-for-data-less-files
//
// The scope is IOPOL_SCOPE_PROCESS rather than IOPOL_SCOPE_THREAD because
// goroutines migrate between OS threads. Thread scope would require
// LockOSThread on every scanning worker. The call needs no privileges and is
// inert on volumes with no dataless files, so it is safe to make it always.
//
// It goes through libSystem rather than a raw syscall because libSystem is the
// only ABI Apple guarantees (QA1118). x/sys/unix has no binding for
// setiopolicy_np, and gdu builds with CGO_ENABLED=0, so purego provides the
// dlopen/dlsym bridge.
//
// Note that with materialization off, reading the contents of a dataless file
// fails with EDEADLK. Scanning only reads metadata, so it is unaffected. The
// file viewer (v) and archive browsing do read contents, and show an error
// dialog for a dataless file. That is intentional: a disk usage tool should not
// download a file to display it.
func PreventDatalessMaterialization() error {
	lib, err := purego.Dlopen(libSystem, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("dlopen %s: %w", libSystem, err)
	}

	// Use Dlsym and RegisterFunc, not RegisterLibFunc, as the latter panics if
	// the symbol is missing.
	sym, err := purego.Dlsym(lib, "setiopolicy_np")
	if err != nil {
		return fmt.Errorf("dlsym setiopolicy_np: %w", err)
	}

	var setiopolicyNp func(iotype, scope, policy int32) int32
	purego.RegisterFunc(&setiopolicyNp, sym)

	if rc := setiopolicyNp(
		iopolTypeVFSMaterializeDatalessFiles,
		iopolScopeProcess,
		iopolMaterializeDatalessFilesOff,
	); rc != 0 {
		return fmt.Errorf("setiopolicy_np returned %d", rc)
	}
	return nil
}
