// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"regexp"
	"strconv"
)

// UI struct
type UI struct {
	Analyzer              Analyzer
	IgnoreDirPaths        map[string]struct{}
	IgnoreDirPathPatterns *regexp.Regexp
	IgnoreHidden          bool
	UseColors             bool
	UseSIPrefix           bool
	ShowProgress          bool
	ShowApparentSize      bool
	ShowRelativeSize      bool
	ConstGC               bool
}

// SetAnalyzer sets analyzer instance
func (ui *UI) SetAnalyzer(a Analyzer) {
	ui.Analyzer = a
}

// SetFollowSymlinks sets whether symlinks to files should be followed
func (ui *UI) SetFollowSymlinks(v bool) {
	ui.Analyzer.SetFollowSymlinks(v)
}

// SetShowAnnexedSize sets whether to use annexed size of git-annex files
func (ui *UI) SetShowAnnexedSize(v bool) {
	ui.Analyzer.SetShowAnnexedSize(v)
}

// SetTimeFilter sets the time filter function for file inclusion
func (ui *UI) SetTimeFilter(timeFilter TimeFilter) {
	ui.Analyzer.SetTimeFilter(timeFilter)
}

// binary multiplies prefixes (IEC)
const (
	_ float64 = 1 << (10 * iota)
	Ki
	Mi
	Gi
	Ti
	Pi
	Ei
)

// SI prefixes
const (
	K float64 = 1e3
	M float64 = 1e6
	G float64 = 1e9
	T float64 = 1e12
	P float64 = 1e15
	E float64 = 1e18
)

// FormatNumber returns number as a string with thousands separator
func FormatNumber(n int64) string {
	in := []byte(strconv.FormatInt(n, 10))

	var out []byte
	if i := len(in) % 3; i != 0 {
		if out, in = append(out, in[:i]...), in[i:]; len(in) > 0 {
			out = append(out, ',')
		}
	}
	for len(in) > 0 {
		if out, in = append(out, in[:3]...), in[3:]; len(in) > 0 {
			out = append(out, ',')
		}
	}
	return string(out)
}
