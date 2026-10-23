package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/device"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/maruel/natural"
	log "github.com/sirupsen/logrus"
)

// SubvolumeAnalyzer scans a directory as its own scope, counting every byte of disk data once.
// Snapshots share their data with each other, so the usage of a file is only the part of its
// extents not referenced by a file scanned earlier. Entries are scanned in natural name order,
// which is chronological for snapper (numbers) and timeshift (timestamps), so the oldest
// snapshot carries the full size and each later one only what changed since.
// It is sequential on purpose: the split between snapshots must be deterministic.
type SubvolumeAnalyzer struct {
	BaseAnalyzer
	extents *device.ExtentSet
}

// CreateSubvolumeAnalyzer returns Analyzer
func CreateSubvolumeAnalyzer() *SubvolumeAnalyzer {
	a := &SubvolumeAnalyzer{extents: device.NewExtentSet()}
	a.Init()
	return a
}

// AnalyzeDir analyzes given path
func (a *SubvolumeAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

	go a.UpdateProgress()
	dir := a.processDir(path)

	dir.BasePath = filepath.Dir(path)
	a.setCurrentDir(dir)

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return dir
}

func (a *SubvolumeAnalyzer) processDir(path string) *Dir {
	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}
	slices.SortFunc(files, func(left, right os.DirEntry) int {
		switch {
		case natural.Less(left.Name(), right.Name()):
			return -1
		case natural.Less(right.Name(), left.Name()):
			return 1
		}
		return 0
	})

	dir := &Dir{
		File: &File{
			Name: filepath.Base(path),
			Flag: getDirFlag(err, len(files)),
		},
		ItemCount: 1,
		Files:     make(fs.Files, 0, len(files)),
	}
	setDirPlatformSpecificAttrs(dir, path)

	var totalUsage int64
	for _, entry := range files {
		if a.IsCancelled() {
			break
		}
		name := entry.Name()
		entryPath := filepath.Join(path, name)

		if entry.IsDir() {
			if a.shouldSkipDir(name, entryPath) {
				continue
			}
			subdir := a.processDir(entryPath)
			subdir.Parent = dir
			dir.AddFile(subdir)
			continue
		}

		if a.ignoreFileType != nil && a.ignoreFileType(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			log.Print(err.Error())
			dir.SetFlag('!')
			continue
		}
		if a.matchesTimeFilterFn != nil && !a.matchesTimeFilterFn(info.ModTime()) {
			continue
		}

		file := &File{
			Name:    name,
			Flag:    getFlag(info),
			Size:    info.Size(),
			Parent:  dir,
			Symlink: readSymlinkTarget(entry.Type(), entryPath),
		}
		setPlatformSpecificAttrs(file, info)
		if info.Mode().IsRegular() {
			a.attributeUsage(file, entryPath, info)
		}
		totalUsage += file.Usage
		dir.AddFile(file)
	}

	a.progressCurrentItemName.Store(path)
	a.progressItemCount.Add(int64(len(files)))
	a.progressTotalUsage.Add(totalUsage)
	return dir
}

// attributeUsage reduces the usage of a regular file to the part of its data not seen yet.
// The share of new data is taken from the file extents and applied to the size the filesystem
// reports for the file, because the extent lengths are uncompressed and say nothing about disk usage.
func (a *SubvolumeAnalyzer) attributeUsage(file *File, path string, info os.FileInfo) {
	extents, err := device.ReadExtents(path)
	if err != nil {
		log.Printf("Cannot read extents of %s, counting its full usage: %s", path, err)
		file.Flag = '!'
		return
	}

	key := device.InlineKey{MtimeNs: info.ModTime().UnixNano(), Size: info.Size()}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		key.Inode = stat.Ino
	}
	fresh, total := a.extents.Add(extents, key)
	if total == 0 {
		return
	}
	file.Usage = int64(float64(file.Usage) * float64(fresh) / float64(total))
}

// AttachSnapshotScope puts a scanned scope in place of the empty placeholder of a skipped
// subvolume in its parent directory. The scope keeps the flag 'S', so the parent never counts it:
// its row shows the size it takes on its own, the rest of the disk stays as it was.
func AttachSnapshotScope(placeholder, scanned fs.Item) error {
	parent, ok := placeholder.GetParent().(*Dir)
	if !ok {
		return fmt.Errorf("parent of %s does not hold the tree in memory", placeholder.GetPath())
	}
	scope, ok := scanned.(*Dir)
	if !ok {
		return fmt.Errorf("scanned %s is not a directory", scanned.GetPath())
	}
	if placeholder.GetFlag() != snapshotFlag {
		return fmt.Errorf("%s is not a skipped subvolume", placeholder.GetPath())
	}

	scope.UpdateStats(make(fs.HardLinkedItems))
	scope.SetFlag(snapshotFlag)
	scope.Parent = parent
	scope.BasePath = filepath.Dir(placeholder.GetPath())

	parent.m.Lock()
	defer parent.m.Unlock()
	index := slices.Index(parent.Files, placeholder)
	if index < 0 {
		return fmt.Errorf("%s is no longer in its parent", placeholder.GetPath())
	}
	parent.Files[index] = scope
	return nil
}

// IsUnscannedSnapshot reports whether item is the empty placeholder of a skipped subvolume
// that was not scanned as its own scope yet.
func IsUnscannedSnapshot(item fs.Item) bool {
	if item.GetFlag() != snapshotFlag || !item.IsDir() {
		return false
	}
	return item.GetSize() == 0 && item.GetUsage() == 0
}

// InSnapshotScope reports whether item is a skipped subvolume or lives inside one. Snapshots are
// read-only and are not removed by deleting files, so callers should refuse to modify these.
func InSnapshotScope(item fs.Item) bool {
	for current := item; current != nil; current = current.GetParent() {
		if current.GetFlag() == snapshotFlag {
			return true
		}
	}
	return false
}
