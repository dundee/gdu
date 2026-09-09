//go:build linux

package device

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestNoCrossUsesMountInfoFilesystem(t *testing.T) {
	root := t.TempDir()
	var stat unix.Stat_t
	require.NoError(t, unix.Stat(root, &stat))
	id := fmt.Sprintf("%d:%d", unix.Major(uint64(stat.Dev)), unix.Minor(uint64(stat.Dev)))
	otherID := fmt.Sprintf("%d:%d", unix.Major(uint64(stat.Dev)), unix.Minor(uint64(stat.Dev))^1)
	rootField := strings.ReplaceAll(root, " ", `\040`)
	input := fmt.Sprintf(`20 1 %s / %s rw shared:1 - ext4 /dev/example rw
21 20 %s /store %s/same ro - ext4 /dev/example rw
22 20 %s / %s/foreign rw - tmpfs tmpfs rw
`, id, rootField, id, rootField, otherID, rootField)

	mounts, err := readMountsFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, mounts, 3)
	assert.Equal(t, "/dev/example", mounts[0].Name)
	assert.Equal(t, root, mounts[0].MountPoint)
	assert.Equal(t, "ext4", mounts[0].Fstype)
	require.NotNil(t, mounts[0].FilesystemID)
	assert.Equal(t, uint64(stat.Dev), *mounts[0].FilesystemID)
	assert.Equal(t, []string{filepath.Join(root, "foreign")}, GetNestedMountpointsPaths(root, mounts))
}

func TestMountInfoEscapedMountpoint(t *testing.T) {
	mounts, err := readMountsFile(strings.NewReader(
		`42 20 0:44 /with\040space /mnt/with\040space ro shared:2 master:1 - nfs server:/export rw`))

	require.NoError(t, err)
	require.Len(t, mounts, 1)
	assert.Equal(t, "server:/export", mounts[0].Name)
	assert.Equal(t, "/mnt/with space", mounts[0].MountPoint)
	assert.Equal(t, "nfs", mounts[0].Fstype)
}

func TestMalformedMountInfoLinesAreSkipped(t *testing.T) {
	mounts, err := readMountsFile(strings.NewReader(`
20 1 0:11
21 1 0:12 / /missing-separator rw
22 1 0:13 / /missing-source rw - ext4
23 1 0:14 / /valid rw - ext4 /dev/example rw
`))

	require.NoError(t, err)
	require.Len(t, mounts, 1)
	assert.Equal(t, "/valid", mounts[0].MountPoint)
}

func TestLegacyMountTableRetainsMountpointExclusion(t *testing.T) {
	root := t.TempDir()
	point := filepath.Join(root, "nested")
	field := strings.ReplaceAll(point, " ", `\040`)
	mounts, err := readMountsFile(strings.NewReader("/dev/example " + field + " ext4 ro 0 0\n"))

	require.NoError(t, err)
	assert.Equal(t, []string{point}, GetNestedMountpointsPaths(root, mounts))
}

func TestUnknownRootFilesystemRetainsMountpointExclusion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	point := filepath.Join(root, "nested")
	id := uint64(1)
	mounts := Devices{{MountPoint: point, FilesystemID: &id}}

	assert.Equal(t, []string{point}, GetNestedMountpointsPaths(root, mounts))
}

func TestParseFilesystemID(t *testing.T) {
	for _, value := range []string{"", "missing", "-1:2", "1:-2", "4294967296:1", "1:4294967296", "1:2:3"} {
		t.Run(value, func(t *testing.T) {
			assert.Nil(t, parseFilesystemID(value))
		})
	}
	id := parseFilesystemID("0:0")
	require.NotNil(t, id)
	assert.Zero(t, *id)
}

func TestNoCrossIgnoresHiddenMountInfoRecords(t *testing.T) {
	root := t.TempDir()
	var stat unix.Stat_t
	require.NoError(t, unix.Stat(root, &stat))
	id := fmt.Sprintf("%d:%d", unix.Major(uint64(stat.Dev)), unix.Minor(uint64(stat.Dev)))
	otherID := fmt.Sprintf("%d:%d", unix.Major(uint64(stat.Dev)), unix.Minor(uint64(stat.Dev))^1)
	rootField := strings.ReplaceAll(root, " ", `\040`)
	input := fmt.Sprintf(`1 1 %s / / rw - ext4 /dev/example rw
22 20 %s / %s/same/hidden rw - tmpfs tmpfs rw
21 20 %s /store %s/same ro - ext4 /dev/example rw
20 1 %s / %s/same rw - tmpfs tmpfs rw
23 21 %s / %s/same/visible rw - tmpfs tmpfs rw
`, id, otherID, rootField, id, rootField, otherID, rootField, otherID, rootField)

	mounts, err := readMountsFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, mounts, 3)
	assert.Equal(t, []string{filepath.Join(root, "same", "visible")}, GetNestedMountpointsPaths(root, mounts))
}

func TestDefaultMountTableHasFilesystemIDs(t *testing.T) {
	mounts, err := Getter.GetMounts()
	require.NoError(t, err)
	for _, mount := range mounts {
		if mount.MountPoint == "/" {
			id, known := getFilesystemID("/")
			require.True(t, known)
			require.NotNil(t, mount.FilesystemID)
			assert.Equal(t, id, *mount.FilesystemID)
			return
		}
	}
	t.Fatal("root mount is missing")
}

func TestMountInfoParentCyclesAreBounded(t *testing.T) {
	mounts, err := readMountsFile(strings.NewReader(`1 2 0:1 / /first rw - tmpfs tmpfs rw
2 1 0:2 / /second rw - tmpfs tmpfs rw
`))

	require.NoError(t, err)
	assert.Len(t, mounts, 2)
}
