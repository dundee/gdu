package ignore

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNilPatternMatchesNothing(t *testing.T) {
	var p *Pattern

	assert.False(t, p.MatchString("/a"))
	assert.Nil(t, New(nil, nil))
	assert.Nil(t, FromRegexp(nil))
}

func TestPatternMatchesTheLastElements(t *testing.T) {
	p := New([][]string{{"node_modules"}, {"*.log"}, {"debug.log*"}, {"build", "Re?ease"}}, nil)

	for path, want := range map[string]bool{
		"/home/node_modules":      true,
		"node_modules":            true,
		`C:\work\node_modules`:    true,
		"/home/node_modules/x":    false,
		"/home/a.log":             true,
		"/home/debug.log.1":       true,
		"/p/build/Release":        true,
		`p\build\Release`:         true,
		"build/Release":           true,
		"/p/Release":              false,
		"Release":                 false,
		"/p/build/Release/x":      false,
		"/p/notbuild/Release":     false,
		"/home/node_modules.json": false,
	} {
		assert.Equal(t, want, p.MatchString(path), path)
	}
}

func TestPatternMatchesItsRegexpToo(t *testing.T) {
	p := New([][]string{{"x"}}, regexp.MustCompile(`^/abs/.*$`))

	assert.True(t, p.MatchString("/abs/dir"))
	assert.True(t, p.MatchString("/y/x"))
	assert.False(t, p.MatchString("/y/z"))
}
