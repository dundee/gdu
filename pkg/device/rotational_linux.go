package device

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	defaultSysfsPath     = "/sys"
	defaultMountInfoPath = "/proc/self/mountinfo"
)

// LinuxRotationalChecker detects whether the block device backing a path is
// rotational (an HDD rather than an SSD).
type LinuxRotationalChecker struct {
	// SysfsPath is the sysfs mount point. An empty value means /sys.
	SysfsPath string
	// MountInfoPath is the kernel mount table. An empty value means
	// /proc/self/mountinfo.
	MountInfoPath string
}

// IsRotational reports whether path is on a rotational block device. The second
// return value is false when the backing device cannot be determined, for
// example for network filesystems or when sysfs is unavailable.
func IsRotational(path string) (rotational, known bool) {
	return LinuxRotationalChecker{}.IsRotational(path)
}

// IsRotational reports whether path is on a rotational block device. The second
// return value is false when the backing device cannot be determined.
func (c LinuxRotationalChecker) IsRotational(path string) (rotational, known bool) {
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		return false, false
	}
	if rotational, known = c.isRotationalDevice(uint64(stat.Dev)); known {
		return rotational, true
	}

	// Some filesystems (notably btrfs) have an anonymous device number, so the
	// filesystem's mount source is the only way to find the backing disk.
	return c.isRotationalMount(path)
}

func (c LinuxRotationalChecker) isRotationalDevice(dev uint64) (rotational, known bool) {
	sysfs := c.SysfsPath
	if sysfs == "" {
		sysfs = defaultSysfsPath
	}

	blockPath := filepath.Join(
		sysfs,
		"dev",
		"block",
		fmt.Sprintf("%d:%d", unix.Major(dev), unix.Minor(dev)),
	)
	resolved, err := filepath.EvalSymlinks(blockPath)
	if err != nil {
		return false, false
	}

	return c.rotationalBlockDir(resolved, make(map[string]bool))
}

func (c LinuxRotationalChecker) isRotationalMount(path string) (rotational, known bool) {
	mountInfoPath := c.MountInfoPath
	if mountInfoPath == "" {
		mountInfoPath = defaultMountInfoPath
	}
	file, err := os.Open(mountInfoPath)
	if err != nil {
		return false, false
	}

	mounts, err := readMountsFile(file)
	if closeErr := file.Close(); closeErr != nil && err == nil {
		return false, false
	}
	if err != nil {
		return false, false
	}

	mountPoint := longestMountPoint(path, mounts)
	if mountPoint == nil {
		return false, false
	}
	return c.rotationalSource(mountPoint.Name)
}

func longestMountPoint(path string, mounts Devices) *Device {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}
	if absPath, err := filepath.Abs(realPath); err == nil {
		realPath = absPath
	}

	var best *Device
	for _, mount := range mounts {
		if mount.MountPoint == "" {
			continue
		}
		relative, err := filepath.Rel(mount.MountPoint, realPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if best == nil || len(mount.MountPoint) > len(best.MountPoint) {
			best = mount
		}
	}
	return best
}

func (c LinuxRotationalChecker) rotationalSource(source string) (rotational, known bool) {
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		resolved = source
	}

	sysfs := c.SysfsPath
	if sysfs == "" {
		sysfs = defaultSysfsPath
	}
	names := []string{filepath.Base(resolved)}
	if resolved != source {
		names = append(names, filepath.Base(source))
	}
	for _, name := range names {
		resolvedBlock, found := lookupBlockDevice(sysfs, name)
		if !found {
			continue
		}
		if rotational, known = c.rotationalBlockDir(resolvedBlock, make(map[string]bool)); known {
			return rotational, true
		}
	}
	return false, false
}

func lookupBlockDevice(sysfs, name string) (string, bool) {
	classDir := filepath.Join(sysfs, "class", "block")
	blockPath := filepath.Join(classDir, name)
	if resolved, err := filepath.EvalSymlinks(blockPath); err == nil {
		return resolved, true
	}

	// The /dev node may be absent (for example in a container) even though
	// the device mapper device is visible in sysfs. Match its user-facing name.
	entries, err := os.ReadDir(classDir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(classDir, entry.Name(), "dm", "name"))
		if err != nil || strings.TrimSpace(string(data)) != name {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(filepath.Join(classDir, entry.Name())); err == nil {
			return resolved, true
		}
	}
	return "", false
}

func (c LinuxRotationalChecker) rotationalBlockDir(dir string, seen map[string]bool) (rotational, known bool) {
	dir = filepath.Clean(dir)
	if seen[dir] {
		return false, false
	}
	seen[dir] = true

	if value, found := rotationalFlag(dir); found {
		if value {
			return true, true
		}
		// A stacked device (dm, md, ...) may not propagate the flag from its
		// members, so keep looking before declaring the device non-rotational.
		if slaveRotational, slaveKnown := c.anySlaveRotational(dir, seen); slaveKnown {
			return slaveRotational, true
		}
		return false, true
	}

	if slaveRotational, slaveKnown := c.anySlaveRotational(dir, seen); slaveKnown {
		return slaveRotational, true
	}

	return false, false
}

// rotationalFlag reads queue/rotational from dir or one of its parents. Sysfs
// keeps the queue for a partition on the parent disk, not on the partition.
func rotationalFlag(dir string) (rotational, known bool) {
	for range 8 {
		data, err := os.ReadFile(filepath.Join(dir, "queue", "rotational"))
		if err == nil {
			return strings.TrimSpace(string(data)) == "1", true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false, false
}

func (c LinuxRotationalChecker) anySlaveRotational(dir string, seen map[string]bool) (rotational, known bool) {
	entries, err := os.ReadDir(filepath.Join(dir, "slaves"))
	if err != nil {
		return false, false
	}

	for _, entry := range entries {
		slavePath, err := filepath.EvalSymlinks(filepath.Join(dir, "slaves", entry.Name()))
		if err != nil {
			continue
		}
		slaveRotational, slaveKnown := c.rotationalBlockDir(slavePath, seen)
		if !slaveKnown {
			continue
		}
		known = true
		if slaveRotational {
			return true, true
		}
	}

	return false, known
}
