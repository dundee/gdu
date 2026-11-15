package cleanup

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// repo is a git work tree found during the walk, with the patterns of the
// .gitignore files seen so far on the current path from its root.
type repo struct {
	root     string
	patterns []gitignore.Pattern
}

// enterDir is called before walking a directory's children. It starts a new
// repo when the directory holds .git and loads its .gitignore. The returned
// func undoes both once the directory is done.
func (w *walker) enterDir(path string, names map[string]struct{}) func() {
	pushed := false
	if _, ok := names[".git"]; ok {
		w.repos = append(w.repos, &repo{root: path})
		pushed = true
	}
	r := w.currentRepo()
	if r == nil {
		return func() {}
	}
	before := len(r.patterns)
	if _, ok := names[".gitignore"]; ok {
		r.patterns = append(r.patterns, readGitignore(filepath.Join(path, ".gitignore"), r.relParts(path))...)
	}
	return func() {
		r.patterns = r.patterns[:before]
		if pushed {
			w.repos = w.repos[:len(w.repos)-1]
		}
	}
}

func (w *walker) currentRepo() *repo {
	if len(w.repos) == 0 {
		return nil
	}
	return w.repos[len(w.repos)-1]
}

func (w *walker) gitIgnored(path string, isDir bool) bool {
	r := w.currentRepo()
	if r == nil || len(r.patterns) == 0 || path == r.root {
		return false
	}
	parts := r.relParts(path)
	if len(parts) == 0 || parts[0] == ".git" {
		return false
	}
	return gitignore.NewMatcher(r.patterns).Match(parts, isDir)
}

func (r *repo) relParts(path string) []string {
	rel, err := filepath.Rel(r.root, path)
	if err != nil || rel == "." {
		return nil
	}
	return strings.Split(filepath.ToSlash(rel), "/")
}

// readGitignore parses one .gitignore file. A file that cannot be read adds
// no patterns: the signal is a hint, not a requirement.
func readGitignore(path string, domain []string) []gitignore.Pattern {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var patterns []gitignore.Pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, domain))
	}
	return patterns
}
