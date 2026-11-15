// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"os"
	"strconv"
	"time"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/internal/ignore"
	"github.com/dundee/gdu/v6/pkg/analyze"
)

// UI is what the frontends share: the analyzer, what a scan leaves out and
// how sizes are shown.
type UI struct {
	Analyzer               analyze.Analyzer
	IgnoreDirPaths         map[string]struct{}
	IgnoreDirPathPatterns  *ignore.Pattern
	IgnoreHidden           bool
	IgnoreOtherFilesystems bool
	IgnoreTypes            []string
	IncludeTypes           []string
	UseColors              bool
	UseSIPrefix            bool
	ShowProgress           bool
	ShowApparentSize       bool
	ShowRelativeSize       bool
	FilteringFiles         bool
	blockSize              int64
	blockSuffix            string
	// otherDevices are the mount points of other devices below the device
	// scanned, see IgnoreOtherDevices
	otherDevices map[string]struct{}
	// scan settings, see analyze.Options
	followSymlinks  bool
	exclusiveSize   bool
	annexedSize     bool
	archiveBrowsing bool
	timeFilter      analyze.TimeFilter
}

// NewUI returns the state the frontends share for the settings s, scanning
// with analyzer.
func NewUI(analyzer analyze.Analyzer, s *config.Settings) *UI {
	ui := &UI{
		Analyzer:               analyzer,
		IgnoreDirPaths:         ignoreDirPathSet(s.Scan.IgnoreDirs),
		IgnoreDirPathPatterns:  s.Scan.IgnoreDirPattern,
		IgnoreHidden:           s.Scan.IgnoreHidden,
		IgnoreOtherFilesystems: s.Scan.IgnoreOtherFilesystems,
		IgnoreTypes:            s.Scan.ExcludeTypes,
		IncludeTypes:           s.Scan.IncludeTypes,
		UseColors:              s.Display.Colors,
		UseSIPrefix:            s.Display.SIPrefix,
		ShowProgress:           s.Display.Progress,
		ShowApparentSize:       s.Display.ApparentSize,
		ShowRelativeSize:       s.Display.RelativeSize,
		followSymlinks:         s.Scan.FollowSymlinks,
		exclusiveSize:          s.Scan.ExclusiveSize,
		annexedSize:            s.Scan.AnnexedSize,
		archiveBrowsing:        s.Scan.ArchiveBrowsing,
	}
	if filter, loc := s.Scan.TimeFilter, s.Scan.TimeLocation; filter != nil {
		ui.timeFilter = func(mtime time.Time) bool {
			return filter.IncludeByTimeFilter(mtime, loc)
		}
		ui.FilteringFiles = true
	}
	return ui
}

// ScanOptions returns the options of a scan with the current settings.
func (ui *UI) ScanOptions() analyze.Options {
	return analyze.Options{
		IgnoreDir:       ui.CreateIgnoreFunc(),
		IgnoreFile:      ui.CreateFileTypeFilter(),
		TimeFilter:      ui.timeFilter,
		FollowSymlinks:  ui.followSymlinks,
		AnnexedSize:     ui.annexedSize,
		ExclusiveSize:   ui.exclusiveSize,
		ArchiveBrowsing: ui.archiveBrowsing,
	}
}

// SetBlockSizeFromEnvironment applies the BLOCK_SIZE or BLOCKSIZE output format.
func (ui *UI) SetBlockSizeFromEnvironment() {
	value, ok := os.LookupEnv("BLOCK_SIZE")
	if !ok {
		value, ok = os.LookupEnv("BLOCKSIZE")
	}
	if !ok {
		return
	}

	switch value {
	case "human-readable":
		return
	case "si":
		ui.UseSIPrefix = true
		return
	}

	blockSize, suffix, ok := parseBlockSize(value)
	if !ok {
		return
	}
	ui.blockSize = blockSize
	ui.blockSuffix = suffix
}

// FormatBlockSize formats size as a rounded-up count of configured blocks.
func (ui *UI) FormatBlockSize(size int64) (string, bool) {
	if ui.blockSize == 0 {
		return "", false
	}

	blocks := size / ui.blockSize
	if size > 0 && size%ui.blockSize != 0 {
		blocks++
	}
	return strconv.FormatInt(blocks, 10) + ui.blockSuffix, true
}

func parseBlockSize(value string) (blockSize int64, suffix string, ok bool) {
	if value == "" {
		return 0, "", false
	}

	index := 0
	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}
	plainUnit := index == 0
	count := int64(1)
	if !plainUnit {
		var err error
		count, err = strconv.ParseInt(value[:index], 10, 64)
		if err != nil || count < 1 {
			return 0, "", false
		}
	}

	unit := value[index:]
	multiplier, valid := blockSizeMultiplier(unit)
	if !valid || count > int64(^uint64(0)>>1)/multiplier {
		return 0, "", false
	}
	if plainUnit {
		suffix = unit
	}

	return count * multiplier, suffix, true
}

func blockSizeMultiplier(unit string) (int64, bool) {
	switch unit {
	case "":
		return 1, true
	case "k", "K", "Ki", "KiB":
		return 1 << 10, true
	case "kB":
		return 1e3, true
	case "M", "Mi", "MiB":
		return 1 << 20, true
	case "MB":
		return 1e6, true
	case "G", "Gi", "GiB":
		return 1 << 30, true
	case "GB":
		return 1e9, true
	case "T", "Ti", "TiB":
		return 1 << 40, true
	case "TB":
		return 1e12, true
	case "P", "Pi", "PiB":
		return 1 << 50, true
	case "PB":
		return 1e15, true
	case "E", "Ei", "EiB":
		return 1 << 60, true
	case "EB":
		return 1e18, true
	default:
		return 0, false
	}
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
