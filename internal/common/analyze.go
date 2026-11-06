// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"os"
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

// Analyzer is type for dir analyzing function
type Analyzer interface {
	AnalyzeDir(path string, ignore ShouldDirBeIgnored, fileTypeFilter ShouldFileBeIgnored) fs.Item
	MeasureSharedUsage(path string, info os.FileInfo, usage int64) (exclusive, shared int64)
	SetFollowSymlinks(bool)
	SetShowAnnexedSize(bool)
	SetExclusiveSize(bool)
	SetTimeFilter(timeFilter TimeFilter)
	SetArchiveBrowsing(bool)
	SetSkipBtrfsSubvolumes(mountPoints []string)
	SetFileTypeFilter(filter ShouldFileBeIgnored)
	Cancel()
	GetDone() SignalGroup
	GetProgress() CurrentProgress
	ResetProgress()
}

// TimeFilter represents a function that determines if a file should be included based on its mtime
type TimeFilter func(mtime time.Time) bool
