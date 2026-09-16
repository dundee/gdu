// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// SetIgnoreFromGitignoreFile sets dirs to ignore from a file with .gitignore-style
// patterns. Blank lines and lines starting with # are skipped; negated patterns
// (starting with !) are not supported and are skipped with a warning.
func (ui *UI) SetIgnoreFromGitignoreFile(ignoreFile string) error {
	var patterns []string
	log.Printf("Reading ignoring dirs in gitignore syntax from file '%s'", ignoreFile)

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
		if strings.HasPrefix(pattern, "!") {
			log.Printf("Negated gitignore pattern %q on line %d of %s is not supported, skipping",
				pattern, lineNo, ignoreFile)
			continue
		}
		regex, err := TranslateGitignorePattern(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %q on line %d of %s: %w",
				pattern, lineNo, ignoreFile, err)
		}
		patterns = append(patterns, regex)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	ui.IgnoreDirPathPatterns, err = regexp.Compile(`^(?:` + strings.Join(patterns, "|") + `)$`)
	return err
}

// TranslateGitignorePattern converts one .gitignore-style pattern into an
// anchored regular expression matched against the whole directory path.
// A trailing / (directory marker) is accepted, a leading / anchors the pattern
// to the scanned root, a name without / matches at any depth, and / in the
// pattern matches either path separator so that patterns written for Unix
// also match Windows paths.
func TranslateGitignorePattern(pattern string) (string, error) {
	// directory marker; gdu ignores only dirs anyway
	pattern = strings.TrimSuffix(pattern, "/")
	// anchor to the scanned root
	rooted := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "./")
	// leading **/ means "at any depth", which is what the non-rooted
	// prefix below already provides
	if strings.HasPrefix(pattern, "**/") {
		pattern = strings.TrimPrefix(pattern, "**/")
		rooted = false
	}
	// gdu skips whole directories, so a/** means "the dir a itself"
	pattern = strings.TrimSuffix(pattern, "/**")

	var expr strings.Builder
	if !rooted {
		expr.WriteString(`(?:.*[\\/])?`)
	}
	for i := 0; i < len(pattern); {
		i = writeGitignorePatternPart(&expr, pattern, i)
	}

	regex := expr.String()
	if _, err := regexp.Compile(regex); err != nil {
		return "", err
	}
	return regex, nil
}

// writeGitignorePatternPart translates the pattern element starting at
// position i (a literal char, glob wildcard, escape or character class) into
// its regular expression form and returns the position of the next element.
func writeGitignorePatternPart(expr *strings.Builder, pattern string, i int) int {
	switch c := pattern[i]; c {
	case '/':
		if i+3 < len(pattern) && pattern[i+1] == '*' && pattern[i+2] == '*' && pattern[i+3] == '/' {
			// /**/ matches zero or more directories
			expr.WriteString(`(?:[\\/].*)?[\\/]`)
			return i + 4
		}
		expr.WriteString(`[\\/]`)
	case '*':
		if i+1 < len(pattern) && pattern[i+1] == '*' {
			expr.WriteString(`.*`)
			return i + 2
		}
		expr.WriteString(`[^\\/]*`)
	case '?':
		expr.WriteString(`[^\\/]`)
	case '\\':
		// gitignore escape, e.g. \# or \ ; the next char is literal
		if i+1 < len(pattern) {
			expr.WriteString(regexp.QuoteMeta(string(pattern[i+1])))
			return i + 2
		}
		expr.WriteString(`\\`)
	case '[':
		return writeGitignoreClass(expr, pattern, i)
	default:
		if strings.ContainsRune(`.+()|^${}]`, rune(c)) {
			expr.WriteByte('\\')
		}
		expr.WriteByte(c)
	}
	return i + 1
}

// writeGitignoreClass translates a [...] character class starting at the
// opening bracket at position i and returns the position right after the
// class; without a closing bracket the '[' is a literal.
func writeGitignoreClass(expr *strings.Builder, pattern string, i int) int {
	j := i + 1
	if j < len(pattern) && (pattern[j] == '!' || pattern[j] == '^') {
		j++
	}
	if j < len(pattern) && pattern[j] == ']' {
		j++
	}
	for j < len(pattern) && pattern[j] != ']' {
		j++
	}
	if j >= len(pattern) {
		expr.WriteString(`\[`)
		return i + 1
	}
	content := pattern[i+1 : j]
	if strings.HasPrefix(content, "!") {
		content = "^" + content[1:]
	}
	expr.WriteString("[" + content + "]")
	return j + 1
}
