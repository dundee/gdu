//go:build linux

package device

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestRotationalDevice(t *testing.T) {
	sysfs := t.TempDir()
	disk := createBlockDevice(t, sysfs, "sda", "1")
	linkBlockDevice(t, sysfs, unix.Mkdev(8, 0), disk)

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.isRotationalDevice(unix.Mkdev(8, 0))

	assert.True(t, rotational)
	assert.True(t, known)
}

func TestNonRotationalDevice(t *testing.T) {
	sysfs := t.TempDir()
	disk := createBlockDevice(t, sysfs, "nvme0n1", "0")
	linkBlockDevice(t, sysfs, unix.Mkdev(259, 0), disk)

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.isRotationalDevice(unix.Mkdev(259, 0))

	assert.False(t, rotational)
	assert.True(t, known)
}

func TestPartitionUsesParentQueue(t *testing.T) {
	sysfs := t.TempDir()
	disk := createBlockDevice(t, sysfs, "sdb", "1")
	partition := filepath.Join(disk, "sdb1")
	require.NoError(t, os.MkdirAll(partition, 0o755))
	linkBlockDevice(t, sysfs, unix.Mkdev(8, 17), partition)

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.isRotationalDevice(unix.Mkdev(8, 17))

	assert.True(t, rotational)
	assert.True(t, known)
}

func TestUnknownDevice(t *testing.T) {
	sysfs := t.TempDir()

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.isRotationalDevice(unix.Mkdev(8, 99))

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestStackedDeviceUsesSlaveFlag(t *testing.T) {
	sysfs := t.TempDir()
	disk := createBlockDevice(t, sysfs, "sdc", "1")
	stacked := createBlockDevice(t, sysfs, "dm-0", "0")
	require.NoError(t, os.MkdirAll(filepath.Join(stacked, "slaves"), 0o755))
	require.NoError(t, os.Symlink(disk, filepath.Join(stacked, "slaves", "sdc")))
	linkBlockDevice(t, sysfs, unix.Mkdev(253, 0), stacked)

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.isRotationalDevice(unix.Mkdev(253, 0))

	assert.True(t, rotational)
	assert.True(t, known)
}

func TestIsRotationalUnknownPath(t *testing.T) {
	checker := LinuxRotationalChecker{SysfsPath: t.TempDir()}
	rotational, known := checker.IsRotational(filepath.Join(t.TempDir(), "missing"))

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestBtrfsStyleMountUsesMountSource(t *testing.T) {
	sysfs := t.TempDir()
	disk := createBlockDevice(t, sysfs, "sda", "1")
	partition := filepath.Join(disk, "sda1")
	require.NoError(t, os.MkdirAll(partition, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "class", "block"), 0o755))
	require.NoError(t, os.Symlink(partition, filepath.Join(sysfs, "class", "block", "sda1")))

	scanDir := t.TempDir()
	source := filepath.Join(t.TempDir(), "sda1")
	require.NoError(t, os.WriteFile(source, nil, 0o600))

	mountInfo := filepath.Join(t.TempDir(), "mountinfo")
	line := fmt.Sprintf("1 1 0:28 / %s rw - btrfs %s rw\n", scanDir, source)
	require.NoError(t, os.WriteFile(mountInfo, []byte(line), 0o600))

	checker := LinuxRotationalChecker{SysfsPath: sysfs, MountInfoPath: mountInfo}
	rotational, known := checker.IsRotational(scanDir)

	assert.True(t, rotational)
	assert.True(t, known)
}

func TestMountSourceDeviceMapperName(t *testing.T) {
	sysfs := t.TempDir()
	stacked := createBlockDevice(t, sysfs, "dm-2", "1")
	require.NoError(t, os.MkdirAll(filepath.Join(stacked, "dm"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stacked, "dm", "name"), []byte("luks\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "class", "block"), 0o755))
	require.NoError(t, os.Symlink(stacked, filepath.Join(sysfs, "class", "block", "dm-2")))

	scanDir := t.TempDir()
	mountInfo := filepath.Join(t.TempDir(), "mountinfo")
	line := fmt.Sprintf("1 1 0:28 / %s rw - btrfs /dev/mapper/luks rw\n", scanDir)
	require.NoError(t, os.WriteFile(mountInfo, []byte(line), 0o600))

	checker := LinuxRotationalChecker{SysfsPath: sysfs, MountInfoPath: mountInfo}
	rotational, known := checker.IsRotational(scanDir)

	assert.True(t, rotational)
	assert.True(t, known)
}

func TestIsRotationalDefaultSysfs(t *testing.T) {
	// Exercises the package-level wrapper and the default /sys path. The
	// result depends on the filesystem backing the test directory.
	_, _ = IsRotational(t.TempDir())
}

func TestIsRotationalUsesStDev(t *testing.T) {
	sysfs := t.TempDir()
	scanDir := t.TempDir()

	var stat unix.Stat_t
	require.NoError(t, unix.Stat(scanDir, &stat))
	disk := createBlockDevice(t, sysfs, "sda", "1")
	linkBlockDevice(t, sysfs, uint64(stat.Dev), disk)

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.IsRotational(scanDir)

	assert.True(t, rotational)
	assert.True(t, known)
}

func TestIsRotationalMountMissingTable(t *testing.T) {
	checker := LinuxRotationalChecker{
		SysfsPath:     t.TempDir(),
		MountInfoPath: filepath.Join(t.TempDir(), "missing"),
	}

	rotational, known := checker.isRotationalMount(t.TempDir())

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestIsRotationalMountUnreadableTable(t *testing.T) {
	checker := LinuxRotationalChecker{
		SysfsPath:     t.TempDir(),
		MountInfoPath: t.TempDir(), // a directory cannot be read as a mount table
	}

	rotational, known := checker.isRotationalMount(t.TempDir())

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestIsRotationalMountNoMatchingMount(t *testing.T) {
	mountInfo := filepath.Join(t.TempDir(), "mountinfo")
	line := "1 1 0:28 / /somewhere/else rw - btrfs /dev/sda1 rw\n"
	require.NoError(t, os.WriteFile(mountInfo, []byte(line), 0o600))

	checker := LinuxRotationalChecker{SysfsPath: t.TempDir(), MountInfoPath: mountInfo}
	rotational, known := checker.isRotationalMount(t.TempDir())

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestIsRotationalMountDefaultTable(t *testing.T) {
	// Exercises the default /proc/self/mountinfo path.
	checker := LinuxRotationalChecker{SysfsPath: t.TempDir()}
	_, _ = checker.isRotationalMount(t.TempDir())
}

func TestLongestMountPointEdgeCases(t *testing.T) {
	root := t.TempDir()
	mounts := Devices{
		{MountPoint: ""},
		{MountPoint: "relative"},
		{MountPoint: root},
	}

	best := longestMountPoint(filepath.Join(root, "missing"), mounts)

	require.NotNil(t, best)
	assert.Equal(t, root, best.MountPoint)
}

func TestRotationalSourceWithoutClassDir(t *testing.T) {
	checker := LinuxRotationalChecker{SysfsPath: t.TempDir()}

	rotational, known := checker.rotationalSource("/missing/device")

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestRotationalSourceNoMatchingDevice(t *testing.T) {
	sysfs := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "class", "block", "other"), 0o755))

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.rotationalSource("missing")

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestRotationalSourceSymlinkFallsBackToSourceName(t *testing.T) {
	sysfs := t.TempDir()
	disk := createBlockDevice(t, sysfs, "sda", "1")
	require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "class", "block"), 0o755))
	require.NoError(t, os.Symlink(disk, filepath.Join(sysfs, "class", "block", "luks")))

	// The resolved target has a different name, so the first lookup misses and
	// the original source name is tried.
	target := filepath.Join(t.TempDir(), "backing")
	require.NoError(t, os.WriteFile(target, nil, 0o600))
	link := filepath.Join(t.TempDir(), "luks")
	require.NoError(t, os.Symlink(target, link))

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.rotationalSource(link)

	assert.True(t, rotational)
	assert.True(t, known)
}

func TestRotationalBlockDirCycle(t *testing.T) {
	dir := t.TempDir()
	seen := map[string]bool{filepath.Clean(dir): true}

	checker := LinuxRotationalChecker{}
	rotational, known := checker.rotationalBlockDir(dir, seen)

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestRotationalBlockDirSlavesOnly(t *testing.T) {
	sysfs := t.TempDir()
	disk := createBlockDevice(t, sysfs, "sda", "1")
	stacked := filepath.Join(sysfs, "devices", "block", "stacked")
	require.NoError(t, os.MkdirAll(filepath.Join(stacked, "slaves"), 0o755))
	require.NoError(t, os.Symlink(disk, filepath.Join(stacked, "slaves", "sda")))

	checker := LinuxRotationalChecker{SysfsPath: sysfs}
	rotational, known := checker.rotationalBlockDir(stacked, make(map[string]bool))

	assert.True(t, rotational)
	assert.True(t, known)
}

func TestRotationalBlockDirUnknown(t *testing.T) {
	checker := LinuxRotationalChecker{}

	rotational, known := checker.rotationalBlockDir(t.TempDir(), make(map[string]bool))

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestRotationalFlagStopsAtRoot(t *testing.T) {
	rotational, known := rotationalFlag("/")

	assert.False(t, rotational)
	assert.False(t, known)
}

func TestAnySlaveRotationalBrokenAndUnknown(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "slaves"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "slaves", "broken")))
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(dir, "slaves", "unknown")))

	checker := LinuxRotationalChecker{}
	rotational, known := checker.anySlaveRotational(dir, make(map[string]bool))

	assert.False(t, rotational)
	assert.False(t, known)
}

func createBlockDevice(t *testing.T, sysfs, name, rotational string) string {
	t.Helper()

	dir := filepath.Join(sysfs, "devices", "block", name)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "queue"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "queue", "rotational"), []byte(rotational+"\n"), 0o600))
	return dir
}

func linkBlockDevice(t *testing.T, sysfs string, dev uint64, target string) {
	t.Helper()

	link := filepath.Join(sysfs, "dev", "block", fmt.Sprintf("%d:%d", unix.Major(dev), unix.Minor(dev)))
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	require.NoError(t, os.Symlink(target, link))
}
