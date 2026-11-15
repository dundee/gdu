// Package ignore matches the paths of the directories a scan leaves out.
package ignore

import (
	"path"
	"regexp"
	"strings"
)

// Pattern matches the paths of the directories a scan leaves out. It is asked
// about every directory of a scan, so it matches cheaply what it can.
//
// Most lines of a .gitignore name a directory wherever it lies, such as
// node_modules, *.log or build/Release. One regular expression of all of
// them has no literal to look for and costs in proportion to the length of
// the path times the number of patterns. Such patterns are matched against
// the last elements of the path instead: a single name by a map lookup, any
// other by path.Match on each element. The other patterns make up one
// regular expression matched against the whole path.
type Pattern struct {
	names map[string]struct{}
	// suffixes are the last elements *suffix, such as *.log
	suffixes []string
	// tails are the patterns of the last elements of a path, each a list of
	// path.Match patterns, the last element last
	tails [][]string
	regex *regexp.Regexp
}

// New returns the pattern matching the paths whose last elements match one
// of tails, element by element, or which regex matches. An element of a tail
// is a path.Match pattern, its * and ? matching no separator; either
// separator, / or \, separates the elements of a path. New returns nil,
// which matches nothing, when there is nothing to match.
func New(tails [][]string, regex *regexp.Regexp) *Pattern {
	if len(tails) == 0 && regex == nil {
		return nil
	}
	p := &Pattern{regex: regex}
	for _, tail := range tails {
		switch {
		case len(tail) == 1 && !strings.ContainsAny(tail[0], metaChars):
			if p.names == nil {
				p.names = make(map[string]struct{})
			}
			p.names[tail[0]] = struct{}{}
		case len(tail) == 1 && strings.HasPrefix(tail[0], "*") && !strings.ContainsAny(tail[0][1:], metaChars):
			p.suffixes = append(p.suffixes, tail[0][1:])
		default:
			p.tails = append(p.tails, tail)
		}
	}
	return p
}

// FromRegexp returns the pattern matching what regex matches, or nil for a
// nil regex.
func FromRegexp(regex *regexp.Regexp) *Pattern {
	return New(nil, regex)
}

// MatchString reports whether p matches the path of a directory. A nil
// pattern matches nothing.
func (p *Pattern) MatchString(dirPath string) bool {
	if p == nil {
		return false
	}
	name := dirPath[lastSeparator(dirPath)+1:]
	if _, ok := p.names[name]; ok {
		return true
	}
	for _, suffix := range p.suffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	for _, tail := range p.tails {
		if matchTail(dirPath, tail) {
			return true
		}
	}
	return p.regex != nil && p.regex.MatchString(dirPath)
}

// matchTail reports whether the last elements of dirPath match tail.
func matchTail(dirPath string, tail []string) bool {
	rest := dirPath
	for i := len(tail) - 1; i >= 0; i-- {
		sep := lastSeparator(rest)
		// New's patterns are valid, so Match fails on none
		if matched, err := path.Match(tail[i], rest[sep+1:]); err != nil || !matched {
			return false
		}
		if sep < 0 {
			return i == 0
		}
		rest = rest[:sep]
	}
	return true
}

// metaChars are the characters with a meaning in a path.Match pattern.
const metaChars = `*?[\`

// lastSeparator returns the index of the last separator of a path, either
// one as the patterns are written for any system, or -1.
func lastSeparator(dirPath string) int {
	return strings.LastIndexAny(dirPath, `/\`)
}
