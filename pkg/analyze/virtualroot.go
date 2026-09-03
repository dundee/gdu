package analyze

import (
	"github.com/dundee/gdu/v5/pkg/fs"
)

// VirtualRootName is the display name of the synthetic directory that groups
// several independently scanned roots.
//
// It is deliberately not a valid absolute path. Every consumer that resolves a
// path back to the filesystem (rescan, delete, spawn shell, change cwd) keys off
// the item's path, so a name that cannot be mistaken for a real directory keeps
// those operations from ever targeting the virtual root itself.
const VirtualRootName = "(multiple)"

// CreateVirtualRootDir groups several independently scanned roots under one
// synthetic parent directory, so that scanning N directories can be presented
// through the same single-rooted tree the rest of gdu expects.
//
// The roots keep their own BasePath, so Dir.GetPath still reports their real
// absolute path even though they now have a parent. The virtual root has
// neither a parent nor a BasePath, which is what IsVirtualRootDir detects and
// what makes GetPath fall back to VirtualRootName.
//
// Stats are not aggregated here; call UpdateStats on the returned dir once the
// roots have been scanned.
func CreateVirtualRootDir(roots ...fs.Item) *Dir {
	dir := &Dir{
		File: &File{
			Name: VirtualRootName,
			Flag: getDirFlag(nil, len(roots)),
		},
		ItemCount: 1,
		Files:     make(fs.Files, 0, len(roots)),
	}

	for _, root := range roots {
		root.SetParent(dir)
		dir.AddFile(root)
	}

	return dir
}

// ItemDisplayName returns the label to show for item when it is listed directly
// under parent.
//
// Scanned roots grouped under the virtual top level dir are labelled with their
// absolute path rather than their base name: base names collide as soon as two
// roots come from different parent directories, and the absolute path is what
// the user asked for on the command line.
func ItemDisplayName(parent, item fs.Item) string {
	if IsVirtualRootDir(parent) {
		return item.GetPath()
	}
	return item.GetName()
}

// IsVirtualRootDir reports whether item is the synthetic dir created by
// CreateVirtualRootDir, i.e. a node with no counterpart on the filesystem.
func IsVirtualRootDir(item fs.Item) bool {
	dir, ok := item.(*Dir)
	if !ok || dir.File == nil {
		return false
	}
	return dir.Parent == nil && dir.BasePath == "" && dir.Name == VirtualRootName
}
