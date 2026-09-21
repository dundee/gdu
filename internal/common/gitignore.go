// Package common contains commong logic and interfaces used across Gdu
// nolint: revive //Why: this is common package
package common

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// SetIgnoreFromGitignoreFile sets dirs to ignore from a file with
// .gitignore-style patterns.
//
// scanRoots are the resolved directories being scanned; patterns anchored with
// a leading / are matched relative to them. Blank lines and # comments are
// skipped. Negated (!) and match-everything patterns are rejected with an
// error rather than skipped, because both would silently make gdu report less
// than is really on disk.
//
// The resulting patterns are added to whatever --ignore-dirs-pattern and
// --ignore-from already contributed; the sources combine.
func (ui *UI) SetIgnoreFromGitignoreFile(ignoreFile string, scanRoots []string) error {
	var fragments []string
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
			return fmt.Errorf(
				"negated pattern %q on line %d of %s is not supported: gdu prunes whole "+
					"directories, so an already ignored directory cannot be un-ignored; "+
					"remove the line or drop the pattern it negates",
				pattern, lineNo, ignoreFile)
		}
		fragment, err := TranslateGitignorePattern(pattern, scanRoots)
		if err != nil {
			return fmt.Errorf("invalid pattern %q on line %d of %s: %w",
				pattern, lineNo, ignoreFile, err)
		}
		fragments = append(fragments, fragment)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return ui.addIgnoreDirPatterns(fragments)
}

// TranslateGitignorePattern converts one .gitignore-style pattern into a
// regular expression fragment matched against the whole directory path.
//
// A trailing / (directory marker) is accepted, a name without / matches at any
// depth, and / in the pattern matches either path separator so that patterns
// written for Unix also match Windows paths. A leading / anchors the pattern to
// scanRoots; with no scanRoots it anchors to the start of the path instead.
func TranslateGitignorePattern(pattern string, scanRoots []string) (string, error) {
	body, rooted := stripGitignoreDecorations(pattern)
	if body == "" {
		return "", errors.New("pattern has no name to match")
	}

	var expr strings.Builder
	switch {
	case !rooted:
		expr.WriteString(`(?:.*[\\/])?`)
	case len(scanRoots) > 0:
		expr.WriteString(scanRootPrefix(scanRoots))
	}
	for i := 0; i < len(body); {
		i = writeGitignorePatternPart(&expr, body, i)
	}

	regex := expr.String()
	compiled, err := regexp.Compile(`^(?:` + regex + `)$`)
	if err != nil {
		return "", err
	}
	if matchesEveryPath(compiled) {
		return "", errors.New(
			"pattern matches every directory, which would hide the whole scan")
	}
	return regex, nil
}

// stripGitignoreDecorations removes the parts of a pattern that carry meaning
// on their own - the directory marker, the root anchor and the ** segments
// that gdu's whole-directory pruning makes redundant - and reports whether
// what is left is anchored to the scanned root.
func stripGitignoreDecorations(pattern string) (body string, rooted bool) {
	// directory marker; gdu ignores only dirs anyway
	body = strings.TrimSuffix(pattern, "/")
	// anchor to the scanned root
	rooted = strings.HasPrefix(body, "/")
	body = strings.TrimPrefix(body, "/")
	body = strings.TrimPrefix(body, "./")
	// leading **/ means "at any depth", which is what the non-rooted
	// prefix already provides
	if strings.HasPrefix(body, "**/") {
		body = strings.TrimPrefix(body, "**/")
		rooted = false
	}
	// gdu skips whole directories, so a/** means "the dir a itself"
	body = strings.TrimSuffix(body, "/**")
	return body, rooted
}

// scanRootPrefix builds the alternation of scanned roots that a pattern
// anchored with a leading / is matched against, followed by a separator.
// gdu resolves every scanned path with filepath.Abs, so anchoring at the start
// of the regex alone would never match anything.
func scanRootPrefix(scanRoots []string) string {
	quoted := make([]string, 0, len(scanRoots))
	for _, root := range scanRoots {
		// a root of "/" (or "C:\") must not contribute its own separator,
		// the one appended below would double it
		trimmed := strings.TrimRight(root, `/\`)
		quoted = append(quoted, regexp.QuoteMeta(trimmed))
	}
	return `(?:` + strings.Join(quoted, "|") + `)[\\/]`
}

// matchesEveryPath reports whether a translated pattern is so broad that it
// would prune the entire scan. Patterns such as "*", "**" or "/**" are legal
// gitignore but meaningless to gdu, and silently returning an empty tree is
// far worse than refusing the pattern. The sentinels use NUL bytes so that no
// realistic pattern can match them by intent.
func matchesEveryPath(compiled *regexp.Regexp) bool {
	return compiled.MatchString("\x00\x01") || compiled.MatchString("\x00\x01/\x02\x03")
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
