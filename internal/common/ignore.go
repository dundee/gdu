// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// CreateIgnorePattern creates one pattern from all path patterns.
// Every relative pattern also gets its absolute form added as an alternative
// and vice versa, so that patterns work no matter whether the scanned paths
// are relative or absolute. The added alternative is the pattern itself with
// a generated path prefix, so it stays a regular expression; only the path
// separators in it are escaped.
func CreateIgnorePattern(paths []string) (compiled *regexp.Regexp, err error) {
	fragments, err := regexPatternFragments(paths)
	if err != nil {
		return nil, err
	}
	return compileIgnoreFragments(fragments)
}

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

// addIgnoreDirPatterns registers regex fragments coming from one pattern
// source and recompiles the combined pattern.
//
// Sources accumulate instead of overriding each other: --ignore-dirs-pattern,
// --ignore-from and --ignore-from-gitignore all feed this one pattern, so a
// pattern the user asked for is never silently dropped because another flag
// was given as well.
func (ui *UI) addIgnoreDirPatterns(fragments []string) error {
	ui.ignorePatternFragments = append(ui.ignorePatternFragments, fragments...)
	compiled, err := compileIgnoreFragments(ui.ignorePatternFragments)
	if err != nil {
		return err
	}
	ui.IgnoreDirPathPatterns = compiled
	return nil
}

// SetIgnoreDirPaths sets paths to ignore
func (ui *UI) SetIgnoreDirPaths(paths []string) {
	log.Printf("Ignoring dirs %s", strings.Join(paths, ", "))
	ui.IgnoreDirPaths = make(map[string]struct{}, len(paths)*2)
	for _, path := range paths {
		ui.IgnoreDirPaths[path] = struct{}{}
		if !filepath.IsAbs(path) {
			if absPath, err := filepath.Abs(path); err == nil {
				ui.IgnoreDirPaths[absPath] = struct{}{}
			}
		} else {
			if relPath, err := filepath.Rel("/", path); err == nil {
				ui.IgnoreDirPaths[relPath] = struct{}{}
			}
		}
	}
}

// SetIgnoreDirPatterns sets regular patterns of dirs to ignore
func (ui *UI) SetIgnoreDirPatterns(paths []string) error {
	log.Printf("Ignoring dir patterns %s", strings.Join(paths, ", "))
	fragments, err := regexPatternFragments(paths)
	if err != nil {
		return err
	}
	return ui.addIgnoreDirPatterns(fragments)
}

// SetIgnoreFromFile sets regular patterns of dirs to ignore.
// The file contains one regular expression per line; blank lines and lines
// starting with # are skipped.
func (ui *UI) SetIgnoreFromFile(ignoreFile string) error {
	var paths []string
	log.Printf("Reading ignoring dir patterns from file '%s'", ignoreFile)

	file, err := os.Open(ignoreFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		pattern := strings.TrimSpace(scanner.Text())
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid pattern %q on line %d of %s: %w",
				pattern, lineNo, ignoreFile, err)
		}
		paths = append(paths, pattern)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	fragments, err := regexPatternFragments(paths)
	if err != nil {
		return err
	}
	return ui.addIgnoreDirPatterns(fragments)
}

// SetIgnoreTypes sets file types to ignore
func (ui *UI) SetIgnoreTypes(types []string) {
	log.Printf("Ignoring file types: %s", strings.Join(types, ", "))
	ui.IgnoreTypes = types
}

// SetIncludeTypes sets file types to include (whitelist)
func (ui *UI) SetIncludeTypes(types []string) {
	log.Printf("Including only file types: %s", strings.Join(types, ", "))
	ui.IncludeTypes = types
}

// SetIgnoreHidden sets flags if hidden dirs should be ignored
func (ui *UI) SetIgnoreHidden(value bool) {
	log.Printf("Ignoring hidden dirs")
	ui.IgnoreHidden = value
}

// ShouldDirBeIgnored returns true if given path should be ignored
func (ui *UI) ShouldDirBeIgnored(name, path string) bool {
	_, shouldIgnore := ui.IgnoreDirPaths[path]
	if shouldIgnore {
		log.Printf("Directory %s ignored", path)
	}
	return shouldIgnore
}

// ShouldDirBeIgnoredUsingPattern returns true if given path should be ignored
func (ui *UI) ShouldDirBeIgnoredUsingPattern(name, path string) bool {
	shouldIgnore := ui.IgnoreDirPathPatterns.MatchString(path)
	if shouldIgnore {
		log.Printf("Directory %s ignored", path)
	}
	return shouldIgnore
}

// IsHiddenDir returns if the dir name begins with dot
func (ui *UI) IsHiddenDir(name, path string) bool {
	shouldIgnore := name[0] == '.'
	if shouldIgnore {
		log.Printf("Directory %s ignored", path)
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
			log.Printf("File %s ignored by type", name)
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

	log.Printf("File %s excluded by type filter", name)
	return false
}

// CreateIgnoreFunc returns function for detecting if dir should be ignored
// nolint: gocyclo // Why: This function is a switch statement that is not too complex
func (ui *UI) CreateIgnoreFunc() ShouldDirBeIgnored {
	switch {
	case len(ui.IgnoreDirPaths) > 0 && ui.IgnoreDirPathPatterns == nil && !ui.IgnoreHidden:
		return ui.ShouldDirBeIgnored
	case len(ui.IgnoreDirPaths) > 0 && ui.IgnoreDirPathPatterns != nil && !ui.IgnoreHidden:
		return func(name, path string) bool {
			return ui.ShouldDirBeIgnored(name, path) || ui.ShouldDirBeIgnoredUsingPattern(name, path)
		}
	case len(ui.IgnoreDirPaths) > 0 && ui.IgnoreDirPathPatterns != nil && ui.IgnoreHidden:
		return func(name, path string) bool {
			return ui.ShouldDirBeIgnored(name, path) || ui.ShouldDirBeIgnoredUsingPattern(name, path) || ui.IsHiddenDir(name, path)
		}
	case len(ui.IgnoreDirPaths) == 0 && ui.IgnoreDirPathPatterns != nil && ui.IgnoreHidden:
		return func(name, path string) bool {
			return ui.ShouldDirBeIgnoredUsingPattern(name, path) || ui.IsHiddenDir(name, path)
		}
	case len(ui.IgnoreDirPaths) == 0 && ui.IgnoreDirPathPatterns != nil && !ui.IgnoreHidden:
		return ui.ShouldDirBeIgnoredUsingPattern
	case len(ui.IgnoreDirPaths) == 0 && ui.IgnoreDirPathPatterns == nil && ui.IgnoreHidden:
		return ui.IsHiddenDir
	case len(ui.IgnoreDirPaths) > 0 && ui.IgnoreDirPathPatterns == nil && ui.IgnoreHidden:
		return func(name, path string) bool {
			return ui.ShouldDirBeIgnored(name, path) || ui.IsHiddenDir(name, path)
		}
	default:
		return func(name, path string) bool { return false }
	}
}

// CreateFileTypeFilter returns function for detecting if file should be ignored based on type
func (ui *UI) CreateFileTypeFilter() ShouldFileBeIgnored {
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
