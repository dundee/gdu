// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// CurrentProgress struct
type CurrentProgress struct {
	CurrentItemName string
	ItemCount       int
	TotalSize       int64
}

// ShouldDirBeIgnored whether path should be ignored
type ShouldDirBeIgnored func(name, path string) bool

// Analyzer is type for dir analyzing function
type Analyzer interface {
	AnalyzeDir(path string, ignore ShouldDirBeIgnored, constGC bool) fs.Item
	SetFollowSymlinks(bool)
	SetShowAnnexedSize(bool)
	SetTimeFilter(timeFilter TimeFilter)
	GetProgressChan() chan CurrentProgress
	GetDone() SignalGroup
	ResetProgress()
}

// TimeFilter represents a function that determines if a file should be included based on its mtime
type TimeFilter func(mtime time.Time) bool
