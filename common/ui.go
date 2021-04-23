package common

import (
	"io/fs"
	"regexp"

	"github.com/dundee/gdu/v4/analyze"
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
