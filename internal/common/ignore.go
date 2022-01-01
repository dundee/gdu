package common

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// CreateIgnorePattern creates one pattern from all path patterns
func CreateIgnorePattern(paths []string) (*regexp.Regexp, error) {
	var err error

	for i, path := range paths {
		if _, err = regexp.Compile(path); err != nil {
			return nil, err
		}
		paths[i] = "(" + path + ")"
	}

	ignore := `^` + strings.Join(paths, "|") + `$`
	return regexp.Compile(ignore)
}

// SetIgnoreDirPaths sets paths to ignore
func (ui *UI) SetIgnoreDirPaths(paths []string) {
	log.Printf("Ignoring dirs %s", strings.Join(paths, ", "))
	ui.IgnoreDirPaths = make(map[string]struct{}, len(paths))
	for _, path := range paths {
		ui.IgnoreDirPaths[path] = struct{}{}
	}
}

// SetIgnoreDirPatterns sets regular patters of dirs to ignore
func (ui *UI) SetIgnoreDirPatterns(paths []string) error {
	var err error
	log.Printf("Ignoring dir patterns %s", strings.Join(paths, ", "))
	ui.IgnoreDirPathPatterns, err = CreateIgnorePattern(paths)
	return err
}

// SetIgnoreFromFile sets regular patters of dirs to ignore
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

// CreateIgnoreFunc returns function for detecting if dir should be ignored
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
