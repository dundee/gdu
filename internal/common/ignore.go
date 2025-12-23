// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// CreateIgnorePattern creates one pattern from all path patterns
func CreateIgnorePattern(paths []string) (compiled *regexp.Regexp, err error) {
	for i, path := range paths {
		if _, err = regexp.Compile(path); err != nil {
			return nil, err
		}
		if !filepath.IsAbs(path) {
			absPath, err := filepath.Abs(path)
			if err == nil {
				paths = append(paths, absPath)
			}
		} else {
			relPath, err := filepath.Rel("/", path)
			if err == nil {
				paths = append(paths, relPath)
			}
		}
		paths[i] = "(" + path + ")"
	}

	ignore := `^` + strings.Join(paths, "|") + `$`
	return regexp.Compile(ignore)
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
	var err error
	log.Printf("Ignoring dir patterns %s", strings.Join(paths, ", "))
	ui.IgnoreDirPathPatterns, err = CreateIgnorePattern(paths)
	return err
}

// SetIgnoreFromFile sets regular patterns of dirs to ignore
func (ui *UI) SetIgnoreFromFile(ignoreFile string) error {
	var err error
	var paths []string
	log.Printf("Reading ignoring dir patterns from file '%s'", ignoreFile)

	file, err := os.Open(ignoreFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		paths = append(paths, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	ui.IgnoreDirPathPatterns, err = CreateIgnorePattern(paths)
	return err
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

// CreateFileTypeFilter returns function for detecting if file should be ignored/included based on type
func (ui *UI) CreateFileTypeFilter() ShouldFileBeFiltered {
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
	
	// No type filtering
	return func(name string) bool { return false }
}
