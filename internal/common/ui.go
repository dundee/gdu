package common

import (
	"io/fs"
	"regexp"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

// UI struct
type UI struct {
	Analyzer              analyze.Analyzer
	IgnoreDirPaths        map[string]struct{}
	IgnoreDirPathPatterns *regexp.Regexp
	IgnoreHidden          bool
	UseColors             bool
	ShowProgress          bool
	ShowApparentSize      bool
	PathChecker           func(string) (fs.FileInfo, error)
}

// file size constants
const (
	_          = iota
	KB float64 = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
)

// file count constants
const (
	K int = 1e3
	M int = 1e6
	G int = 1e9
)
