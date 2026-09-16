package common_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestTranslateGitignorePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		matches []string
		skips   []string
	}{
		{
			name:    "bare name matches at any depth with both separators",
			pattern: "node_modules",
			matches: []string{"node_modules", "a/node_modules", `a\b\node_modules`},
			skips:   []string{"node_modules2", "a/node_modules2", "xxnode_modules"},
		},
		{
			name:    "trailing slash is a dir marker",
			pattern: "target/",
			matches: []string{"target", "x/target", `x\y\target`},
			skips:   []string{"target2"},
		},
		{
			name:    "leading slash anchors to the scanned root",
			pattern: "/build",
			matches: []string{"build"},
			skips:   []string{"a/build", "builder"},
		},
		{
			name:    "star does not cross separators",
			pattern: "*.log",
			matches: []string{"debug.log", "logs/debug.log", `a\b\debug.log`},
			skips:   []string{"log", "logs/debugtxt"},
		},
		{
			name:    "star inside a path",
			pattern: "logs/*.tmp",
			matches: []string{"logs/a.tmp", "x/logs/a.tmp"},
			skips:   []string{"logs/sub/a.tmp", "logs/atmp"},
		},
		{
			name:    "leading **/ means any depth",
			pattern: "**/logs",
			matches: []string{"logs", "a/logs", `a\b\logs`},
			skips:   []string{"logs2"},
		},
		{
			name:    "middle /**/ matches zero or more dirs",
			pattern: "a/**/b",
			matches: []string{"a/b", "a/x/b", "a/x/y/b", `a\x\y\b`},
			skips:   []string{"ab", "a"},
		},
		{
			name:    "trailing /** prunes the dir itself",
			pattern: "build/**",
			matches: []string{"build", "x/build"},
			skips:   []string{"builder"},
		},
		{
			name:    "question mark matches one char",
			pattern: "debug?.log",
			matches: []string{"debug1.log"},
			skips:   []string{"debug12.log", "debug.log"},
		},
		{
			name:    "character class",
			pattern: "[abc]ache",
			matches: []string{"bache"},
			skips:   []string{"dache", "ache"},
		},
		{
			name:    "negated character class",
			pattern: "[!a]b",
			matches: []string{"xb"},
			skips:   []string{"ab"},
		},
		{
			name:    "double star crosses separators",
			pattern: "a**b",
			matches: []string{"axb", "ab", "a/x/b", `a\x\b`},
			skips:   []string{"b", "ba"},
		},
		{
			name:    "trailing escape matches a literal backslash",
			pattern: `foo\`,
			matches: []string{`foo\`},
			skips:   []string{"foo", `foo\bar`},
		},
		{
			name:    "class starting with a closing bracket",
			pattern: "[]]x",
			matches: []string{"]x", "a/]x"},
			skips:   []string{"yx", "x"},
		},
		{
			name:    "unclosed character class is a literal bracket",
			pattern: "foo[bar",
			matches: []string{"foo[bar"},
			skips:   []string{"fooxbar", "foobar"},
		},
		{
			name:    "regex metachars are literal",
			pattern: "c++ (x86)",
			matches: []string{"c++ (x86)"},
			skips:   []string{"c++a (x86)"},
		},
		{
			name:    "escaped hash",
			pattern: `\#weird`,
			matches: []string{"#weird"},
			skips:   []string{"weird"},
		},
		{
			name:    "/**/ after root anchor matches any depth incl. root",
			pattern: "/**/x",
			matches: []string{"x", "a/x"},
			skips:   []string{"ax", "a/x2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex, err := common.TranslateGitignorePattern(tt.pattern)
			assert.Nil(t, err)
			// the fragment is meant to be used inside an anchored alternation
			re, err := regexp.Compile(`^(?:` + regex + `)$`)
			assert.Nil(t, err)

			for _, m := range tt.matches {
				assert.True(t, re.MatchString(m), "%s should match %q", tt.pattern, m)
			}
			for _, s := range tt.skips {
				assert.False(t, re.MatchString(s), "%s should not match %q", tt.pattern, s)
			}
		})
	}
}

func TestSetIgnoreFromGitignoreFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	content := "# comment line\n\nnode_modules/\ntarget\n/build\n*.class\n!keep\n"
	err := os.WriteFile(path, []byte(content), 0o600)
	assert.Nil(t, err)

	ui := &common.UI{}
	err = ui.SetIgnoreFromGitignoreFile(path)
	assert.Nil(t, err)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("node_modules", "a/node_modules"))
	assert.True(t, shouldBeIgnored("node_modules", `a\b\node_modules`))
	assert.True(t, shouldBeIgnored("target", "x/y/target"))
	assert.True(t, shouldBeIgnored("build", "build"))
	assert.False(t, shouldBeIgnored("build", "deep/build"))
	assert.False(t, shouldBeIgnored("xxx", "xxx"))
	// negated patterns are skipped, they do not ignore anything
	assert.False(t, shouldBeIgnored("keep", "keep"))
}

func TestSetIgnoreFromGitignoreInvalidPattern(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	content := "/aaa\nfoo[z-a]bar\n"
	err := os.WriteFile(path, []byte(content), 0o600)
	assert.Nil(t, err)

	ui := &common.UI{}
	err = ui.SetIgnoreFromGitignoreFile(path)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "line 2")
	assert.Contains(t, err.Error(), "foo[z-a]bar")
	assert.Contains(t, err.Error(), path)
}

func TestSetIgnoreFromNotExistingGitignoreFile(t *testing.T) {
	ui := &common.UI{}
	err := ui.SetIgnoreFromGitignoreFile(filepath.Join("xxx", "yyy"))
	assert.NotNil(t, err)
}

func TestSetIgnoreFromGitignoreDirectory(t *testing.T) {
	ui := &common.UI{}
	err := ui.SetIgnoreFromGitignoreFile(t.TempDir())
	assert.NotNil(t, err)
}

func TestSetIgnoreFromEmptyGitignoreFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	err := os.WriteFile(path, []byte(""), 0o600)
	assert.Nil(t, err)

	ui := &common.UI{}
	err = ui.SetIgnoreFromGitignoreFile(path)
	assert.Nil(t, err)

	shouldBeIgnored := ui.CreateIgnoreFunc()
	assert.False(t, shouldBeIgnored("anything", "/anything"))
}
