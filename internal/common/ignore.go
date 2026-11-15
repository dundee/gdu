// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dundee/gdu/v6/internal/ignore"
	"github.com/dundee/gdu/v6/pkg/analyze"
)

// regexPatternFragments validates the given regular path patterns and returns
// them as regex fragments. Each pattern gets an absolute/relative twin so that
// it matches regardless of how the scanned path was spelled.
func regexPatternFragments(paths []string) ([]string, error) {
	fragments := make([]string, 0, len(paths)*2)
	for _, path := range paths {
		if _, err := regexp.Compile(path); err != nil {
			return nil, err
		}
		fragments = append(fragments, "("+path+")")

		// the twin is the generated absolute (or relative) form of the same
		// pattern, so only the path separators are escaped: the user's
		// pattern has to keep working as a regexp inside it, while a Windows
		// prefix like E:\repos\git must not be read as regexp escapes
		if !filepath.IsAbs(path) {
			if absPath, err := filepath.Abs(path); err == nil {
				fragments = append(fragments, "(?:"+escapePathSeparators(absPath)+")")
			}
		} else {
			if relPath, err := filepath.Rel("/", path); err == nil {
				fragments = append(fragments, "(?:"+escapePathSeparators(relPath)+")")
			}
		}
	}
	return fragments, nil
}

// escapePathSeparators doubles backslashes so that a filesystem path is a
// valid (literal) regular expression; on Windows, raw backslashes in paths
// like `E:\repos\git` would otherwise be parsed as invalid escape sequences.
func escapePathSeparators(path string) string {
	return strings.ReplaceAll(path, `\`, `\\`)
}

// compileIgnoreFragments joins regex fragments into a single pattern that
// matches only when the whole path equals one of the alternatives.
// No fragments means no filtering at all, which is a nil pattern rather than
// an empty alternation - `^(?:)$` would still match the empty path and would
// push CreateIgnoreFunc onto the regex branch for no benefit.
func compileIgnoreFragments(fragments []string) (*regexp.Regexp, error) {
	if len(fragments) == 0 {
		return nil, nil
	}
	return regexp.Compile(`^(?:` + strings.Join(fragments, "|") + `)$`)
}

// ignoreDirPathSet returns the set of the paths of the directories to leave
// out, each also in its absolute or relative form, so that it matches however
// the scanned path is spelled.
func ignoreDirPathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths)*2)
	for _, path := range paths {
		set[path] = struct{}{}
		if !filepath.IsAbs(path) {
			if absPath, err := filepath.Abs(path); err == nil {
				set[absPath] = struct{}{}
			}
		} else {
			if relPath, err := filepath.Rel("/", path); err == nil {
				set[relPath] = struct{}{}
			}
		}
	}
	return set
}

// readIgnoreFile returns the patterns of the file ignoreFile, one regular
// expression per line, as regex fragments. Blank lines and # comments are
// skipped.
func readIgnoreFile(ignoreFile string) ([]string, error) {
	var paths []string

	file, err := os.Open(ignoreFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		pattern := strings.TrimSpace(scanner.Text())
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return nil, fmt.Errorf("invalid pattern %q on line %d of %s: %w",
				pattern, lineNo, ignoreFile, err)
		}
		paths = append(paths, pattern)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return regexPatternFragments(paths)
}

// IgnorePattern compiles the patterns of the directories a scan leaves out,
// from their three sources combined: regular expressions (patterns), a file
// of them (ignoreFile) and a file in .gitignore syntax (gitignoreFile), whose
// leading / anchors a pattern at the scanned roots. It returns nil when there
// is none.
func IgnorePattern(patterns []string, ignoreFile, gitignoreFile string, roots []string) (*ignore.Pattern, error) {
	fragments, err := regexPatternFragments(patterns)
	if err != nil {
		return nil, err
	}
	var tails [][]string
	if ignoreFile != "" {
		fromFile, err := readIgnoreFile(ignoreFile)
		if err != nil {
			return nil, err
		}
		fragments = append(fragments, fromFile...)
	}
	if gitignoreFile != "" {
		fromGitignore, err := readGitignoreFile(gitignoreFile, roots)
		if err != nil {
			return nil, err
		}
		tails = fromGitignore.tails
		fragments = append(fragments, fromGitignore.fragments...)
	}
	regex, err := compileIgnoreFragments(fragments)
	if err != nil {
		return nil, err
	}
	return ignore.New(tails, regex), nil
}

// IgnoreOtherDevices leaves the directories at paths, the mount points of
// other devices below the device to scan, out of the scans that follow. They
// replace those given before, and add to the directories and patterns the
// user left out.
func (ui *UI) IgnoreOtherDevices(paths []string) {
	ui.otherDevices = make(map[string]struct{}, len(paths))
	for _, path := range paths {
		ui.otherDevices[path] = struct{}{}
	}
}

// ShouldDirBeIgnored returns true if given path should be ignored
func (ui *UI) ShouldDirBeIgnored(name, path string) bool {
	_, shouldIgnore := ui.IgnoreDirPaths[path]
	if !shouldIgnore {
		_, shouldIgnore = ui.otherDevices[path]
	}
	if shouldIgnore {
		slog.Info("directory ignored", "path", path)
	}
	return shouldIgnore
}

// ShouldDirBeIgnoredUsingPattern returns true if given path should be ignored
func (ui *UI) ShouldDirBeIgnoredUsingPattern(name, path string) bool {
	shouldIgnore := ui.IgnoreDirPathPatterns.MatchString(path)
	if shouldIgnore {
		slog.Info("directory ignored", "path", path)
	}
	return shouldIgnore
}

// IsHiddenDir returns if the dir name begins with dot
func (ui *UI) IsHiddenDir(name, path string) bool {
	shouldIgnore := name[0] == '.'
	if shouldIgnore {
		slog.Info("directory ignored", "path", path)
	}
	return shouldIgnore
}

// ShouldFileBeIgnoredByType returns true if file should be ignored based on its extension
func (ui *UI) ShouldFileBeIgnoredByType(name string) bool {
	if len(ui.IgnoreTypes) == 0 {
		return false
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false // No extension, don't ignore
	}

	// Remove leading dot from extension
	ext = strings.TrimPrefix(ext, ".")

	for _, ignoreType := range ui.IgnoreTypes {
		// Remove leading dot from ignoreType
		cleanIgnoreType := strings.TrimPrefix(strings.ToLower(ignoreType), ".")
		if cleanIgnoreType == ext {
			slog.Info("file ignored by type", "name", name)
			return true
		}
	}
	return false
}

// ShouldFileBeIncludedByType returns true if file should be included based on its extension
func (ui *UI) ShouldFileBeIncludedByType(name string) bool {
	if len(ui.IncludeTypes) == 0 {
		return true // No include filter, include all
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false // No extension, don't include if we have include filter
	}

	// Remove leading dot from extension
	ext = strings.TrimPrefix(ext, ".")

	for _, includeType := range ui.IncludeTypes {
		// Remove leading dot from includeType
		cleanIncludeType := strings.TrimPrefix(strings.ToLower(includeType), ".")
		if cleanIncludeType == ext {
			return true
		}
	}

	slog.Info("file excluded by type filter", "name", name)
	return false
}

// CreateIgnoreFunc returns function for detecting if dir should be ignored
func (ui *UI) CreateIgnoreFunc() analyze.ShouldDirBeIgnored {
	ignoreDir := ui.createPathIgnoreFunc()
	if !ui.IgnoreOtherFilesystems {
		return ignoreDir
	}
	filesystems := newFilesystemIDCache()
	return func(name, path string) bool {
		return ignoreDir(name, path) || filesystems.isOnOtherFilesystem(path)
	}
}

// createPathIgnoreFunc returns function for detecting if dir should be ignored
// based on its name or path only
func (ui *UI) createPathIgnoreFunc() analyze.ShouldDirBeIgnored {
	var checks []analyze.ShouldDirBeIgnored
	if len(ui.IgnoreDirPaths) > 0 || len(ui.otherDevices) > 0 {
		checks = append(checks, ui.ShouldDirBeIgnored)
	}
	if ui.IgnoreDirPathPatterns != nil {
		checks = append(checks, ui.ShouldDirBeIgnoredUsingPattern)
	}
	if ui.IgnoreHidden {
		checks = append(checks, ui.IsHiddenDir)
	}
	switch len(checks) {
	case 0:
		return func(string, string) bool { return false }
	case 1:
		return checks[0]
	}
	return func(name, path string) bool {
		for _, check := range checks {
			if check(name, path) {
				return true
			}
		}
		return false
	}
}

// CreateFileTypeFilter returns function for detecting if file should be ignored based on type
func (ui *UI) CreateFileTypeFilter() analyze.ShouldFileBeIgnored {
	// If we have include types, use whitelist mode
	if len(ui.IncludeTypes) > 0 {
		return func(name string) bool {
			return !ui.ShouldFileBeIncludedByType(name)
		}
	}

	// If we have ignore types, use blacklist mode
	if len(ui.IgnoreTypes) > 0 {
		return func(name string) bool {
			return ui.ShouldFileBeIgnoredByType(name)
		}
	}

	// No type filtering - return nil to indicate no filtering is needed
	return nil
}

// IsFilteringFiles returns true if we have any file type filters set
func (ui *UI) IsFilteringFiles() bool {
	return len(ui.IgnoreTypes) > 0 || len(ui.IncludeTypes) > 0 || ui.FilteringFiles
}
