package device

import "testing"

func TestIsNestedBtrfsSubvolumeFalseForPlainDir(t *testing.T) {
	if IsNestedBtrfsSubvolume(t.TempDir(), map[string]struct{}{}) {
		t.Error("plain dir is not a subvolume root")
	}
}
