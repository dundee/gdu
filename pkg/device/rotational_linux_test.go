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
