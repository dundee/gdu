package analyze

import (
	"context"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// Analyzer scans directories.
type Analyzer interface {
	// AnalyzeDir scans the directory at path. Cancelling ctx stops the scan,
	// which then returns what it found so far. An analyzer runs one scan at
	// a time.
	AnalyzeDir(ctx context.Context, path string, opts Options) fs.Item
	// GetProgress returns the progress of the running scan, or of the last
	// one. It is safe to call during a scan.
	GetProgress() Progress
}

// Options are the settings of one scan.
type Options struct {
	// IgnoreDir tells which directories to leave out; nil leaves out none
	IgnoreDir ShouldDirBeIgnored
	// IgnoreFile tells which files to leave out by their name; nil leaves
	// out none
	IgnoreFile ShouldFileBeIgnored
	// TimeFilter tells which files to count by their mtime; nil counts all
	TimeFilter TimeFilter
	// FollowSymlinks counts the files symlinks point to instead of the links
	FollowSymlinks bool
	// AnnexedSize counts git-annex files by the size of their content
	AnnexedSize bool
	// ExclusiveSize counts only the bytes of a file not shared with other
	// files (Linux and macOS)
	ExclusiveSize bool
	// ArchiveBrowsing reads zip and tar archives as directories
	ArchiveBrowsing bool
}

// FilteringFiles reports whether the scan leaves out files by their name or
// mtime, which changes how the totals of directories left empty are counted:
// pass it to UpdateStats.
func (o Options) FilteringFiles() bool {
	return o.IgnoreFile != nil || o.TimeFilter != nil
}

// ShouldDirBeIgnored reports whether the directory named name, at path, is
// left out of a scan.
type ShouldDirBeIgnored func(name, path string) bool

// ShouldFileBeIgnored reports whether the file named name is left out of a
// scan.
type ShouldFileBeIgnored func(name string) bool

// TimeFilter reports whether a file with the modification time mtime is
// counted by a scan.
type TimeFilter func(mtime time.Time) bool

// Progress is how far a scan has got.
type Progress struct {
	// CurrentItemName is the path of the directory read last
	CurrentItemName string
	// ItemCount is the number of items found so far
	ItemCount int64
	// TotalUsage is the disk usage of the files found so far
	TotalUsage int64
}
