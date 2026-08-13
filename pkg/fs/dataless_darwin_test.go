//go:build darwin

package fs

import (
	"testing"

	"golang.org/x/sys/unix"
)

// TestPreventDatalessMaterialization checks that the call reaches the kernel, by
// reading back the state it is supposed to change.
//
// setiopolicy_np with IOPOL_TYPE_VFS_MATERIALIZE_DATALESS_FILES and the
// vfs.nspace.prevent_materialization sysctl are the same per-process state.
// x/sys/unix exposes sysctl readers but no writer, which is enough here: we can
// verify the result even though we cannot produce it.
//
// The state is per-process and has no effect outside the test binary.
func TestPreventDatalessMaterialization(t *testing.T) {
	const preventMaterializationSysctl = "vfs.nspace.prevent_materialization"

	if err := PreventDatalessMaterialization(); err != nil {
		t.Fatalf("PreventDatalessMaterialization: %s", err)
	}

	after, err := unix.SysctlUint32(preventMaterializationSysctl)
	if err != nil {
		t.Skipf("cannot read %s: %s", preventMaterializationSysctl, err)
	}
	if after != 1 {
		t.Errorf("%s = %d, want 1", preventMaterializationSysctl, after)
	}
}
