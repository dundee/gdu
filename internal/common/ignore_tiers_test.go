package common_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/internal/common"
)

// nodeGitignore is GitHub's .gitignore template for Node, without its
// negations, which gdu refuses.
var nodeGitignore = []string{
	"logs", "*.log", "npm-debug.log*", "yarn-debug.log*", "yarn-error.log*",
	"lerna-debug.log*", ".pnpm-debug.log*", "report.[0-9]*.[0-9]*.[0-9]*.[0-9]*.json",
	"pids", "*.pid", "*.seed", "*.pid.lock", "lib-cov", "coverage", "*.lcov",
	".nyc_output", ".grunt", "bower_components", ".lock-wscript", "build/Release",
	"node_modules/", "jspm_packages/", "web_modules/", "*.tsbuildinfo", ".npm",
	".eslintcache", ".stylelintcache", ".rpt2_cache/", ".rts2_cache_cjs/",
	".rts2_cache_es/", ".rts2_cache_umd/", ".node_repl_history", "*.tgz",
	".yarn-integrity", ".env", ".env.development.local", ".env.test.local",
	".env.production.local", ".env.local", ".cache", ".parcel-cache", ".next",
	"out", ".nuxt", "dist", ".cache/", ".vuepress/dist", ".temp", ".docusaurus",
	".serverless/", ".fusebox/", ".dynamodb/", ".tern-port", ".vscode-test",
	".yarn/cache", ".yarn/unplugged", ".yarn/build-state.yml",
	".yarn/install-state.gz", ".pnp.*",
}

// a path of about 110 characters, matching none of the patterns, as most
// directories of a scan do
const benchPath = "/Users/someone/work/projects/some-project/packages/frontend/src/components/widgets/datagrid/internal"

func BenchmarkIgnoreGitignore(b *testing.B) {
	file := filepath.Join(b.TempDir(), ".gitignore")
	require.NoError(b, os.WriteFile(file, []byte(strings.Join(nodeGitignore, "\n")), 0o600))
	pattern, err := common.IgnorePattern(nil, "", file, []string{"/Users/someone/work"})
	require.NoError(b, err)

	for b.Loop() {
		if pattern.MatchString(benchPath) {
			b.Fatal("matched")
		}
	}
}

func BenchmarkIgnoreRegexps(b *testing.B) {
	pattern, err := common.IgnorePattern([]string{".*node_modules", `.*\.git`, ".*/target"}, "", "", nil)
	require.NoError(b, err)

	for b.Loop() {
		if pattern.MatchString(benchPath) {
			b.Fatal("matched")
		}
	}
}

// The patterns matched against the last elements of a path match what the
// regular expression of each pattern matches.
func TestGitignoreTailsMatchAsTheirRegexps(t *testing.T) {
	patterns := append([]string{
		"node_modules", "node_modules/", "**/node_modules", "*.log", "npm-debug.log*",
		"build/Release", ".yarn/cache", "a/*/c", "a?c", "x[0-9]y", "x[!0-9]y", "x[^0-9]y",
		"foo.bar", "a+b", "(paren)", "{brace}", "a]b", "[unclosed", `\#hash`, "/rooted",
		"rooted/sub/", "a/**/b", "a/**", "**/a/b", "./dot", "*.tar.*", "?",
	}, nodeGitignore...)
	paths := []string{
		"/home/u/node_modules", "node_modules", "/home/u/node_modulesx", `C:\work\node_modules`,
		"/a/x.log", "/a/x.log/y", "/x/npm-debug.log.1", "/p/build/Release", "/p/Release",
		"build/Release", `p\build\Release`, "/x/.yarn/cache", "/a/b/c", "/q/a/b/c", "/a/c",
		"/abc", "/x5y", "/xay", "/x/y", "/foo.bar", "/fooxbar", "/a+b", "/(paren)", "/{brace}",
		"/a]b", "/[unclosed", "/#hash", "/rooted", "/home/u/rooted/sub", "/a/b", "/a/z/b",
		"/a", "/dot", "/x.tar.gz", "/b", "/home/line\nbreak/node_modules", "", "/",
		"/report.1.2.3.4.json", "/report.a.b.c.d.json", "/.env.local", "/.pnp.cjs",
	}
	roots := []string{"/home/u"}
	dir := t.TempDir()

	for i, pattern := range patterns {
		file := filepath.Join(dir, fmt.Sprintf("%d.gitignore", i))
		require.NoError(t, os.WriteFile(file, []byte(pattern+"\n"), 0o600))
		matcher, err := common.IgnorePattern(nil, "", file, roots)
		if err != nil {
			continue // refused, such as a pattern matching every path
		}
		fragment, err := common.TranslateGitignorePattern(pattern, roots)
		require.NoError(t, err)
		regex := regexp.MustCompile(`^(?:` + fragment + `)$`)

		for _, dirPath := range paths {
			if strings.Contains(dirPath, "\n") {
				// a regexp's . matches no newline; the tails do, as git
				continue
			}
			if strings.Contains(pattern, "[!") || strings.Contains(pattern, "[^") {
				if dirPath == "/x/y" {
					// a negated class of the regexp matches a separator;
					// in git, as in the tails, it does not
					assert.False(t, matcher.MatchString(dirPath), "%q on %q", pattern, dirPath)
					continue
				}
			}
			assert.Equal(t, regex.MatchString(dirPath), matcher.MatchString(dirPath), "%q on %q", pattern, dirPath)
		}
	}
}

// A pattern naming a directory wherever it lies matches it under a directory
// whose name holds a newline, as git does.
func TestGitignoreTailsMatchUnderAnyName(t *testing.T) {
	file := filepath.Join(t.TempDir(), ".gitignore")
	require.NoError(t, os.WriteFile(file, []byte("node_modules\n*.log\n"), 0o600))
	matcher, err := common.IgnorePattern(nil, "", file, nil)
	require.NoError(t, err)

	assert.True(t, matcher.MatchString("/home/line\nbreak/node_modules"))
	assert.True(t, matcher.MatchString("/home/line\nbreak/x.log"))
}
