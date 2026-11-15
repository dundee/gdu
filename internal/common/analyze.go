// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"context"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// CurrentProgress struct
type CurrentProgress struct {
	CurrentItemName string
	ItemCount       int64
	TotalUsage      int64
}

// ShouldDirBeIgnored whether path should be ignored
type ShouldDirBeIgnored func(name, path string) bool

// ShouldFileBeIgnored whether file should be ignored based on type
type ShouldFileBeIgnored func(name string) bool

// ScanOptions are the settings of one scan.
type ScanOptions struct {
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
// mtime, which changes how the totals of directories left empty are counted.
func (o ScanOptions) FilteringFiles() bool {
	return o.IgnoreFile != nil || o.TimeFilter != nil
}

// Analyzer scans directories.
type Analyzer interface {
	// AnalyzeDir scans the directory at path. Cancelling ctx stops the scan,
	// which then returns what it found so far. An analyzer runs one scan at
	// a time.
	AnalyzeDir(ctx context.Context, path string, opts ScanOptions) fs.Item
	// GetProgress returns the progress of the running scan, or of the last
	// one. It is safe to call during a scan.
	GetProgress() CurrentProgress
}

// TimeFilter represents a function that determines if a file should be included based on its mtime
type TimeFilter func(mtime time.Time) bool
