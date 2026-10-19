package cleanup

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// Config is the user-provided part of the classification, read from the
// "cleanup" section of the gdu config file.
type Config struct {
	// Exclude lists paths that are never candidates, nor anything below them.
	Exclude []string `yaml:"exclude"`
	// Rules are evaluated before DefaultRules.
	Rules []Rule `yaml:"rules"`
}

// Classifier assigns categories to items. It is immutable and safe for
// concurrent use.
type Classifier struct {
	rules    []Rule
	excludes []string
}

// NewClassifier validates the user's config and combines it with
// DefaultRules. home expands "~/" in paths.
func NewClassifier(cfg Config, home string) (*Classifier, error) {
	c := &Classifier{}
	for i := range cfg.Rules {
		r := cfg.Rules[i]
		if r.Confidence == "" {
			r.Confidence = Review
		}
		if err := r.validate(); err != nil {
			return nil, fmt.Errorf("cleanup rule %d: %w", i+1, err)
		}
		c.rules = append(c.rules, r)
	}
	c.rules = append(c.rules, DefaultRules...)
	for i := range c.rules {
		c.rules[i].compile(home)
	}
	for _, path := range cfg.Exclude {
		c.excludes = append(c.excludes, filepath.Clean(expandHome(path, home)))
	}
	return c, nil
}

// NewDefaultClassifier applies DefaultRules only.
func NewDefaultClassifier(home string) *Classifier {
	c, err := NewClassifier(Config{}, home)
	if err != nil {
		// only user rules are validated, so this cannot happen
		panic(err)
	}
	return c
}

func (r *Rule) compile(home string) {
	if r.Path != "" {
		r.Path = filepath.Clean(expandHome(r.Path, home))
	}
	if r.Under != "" {
		r.Under = filepath.Clean(expandHome(r.Under, home))
	}
}

// Match is the classification of one item.
type Match struct {
	Category   Category
	Confidence Confidence
	Hint       string
	// Inherited is set for items inside a matched directory: they are
	// candidates as part of it, not on their own.
	Inherited bool
}

// Amount is an amount of space in both metrics.
type Amount struct {
	Usage int64 `json:"usage"`
	Size  int64 `json:"size"`
}

// Totals is the space taken by each category.
type Totals map[Category]Amount

func (t Totals) add(other Totals) Totals {
	if len(other) == 0 {
		return t
	}
	if t == nil {
		t = make(Totals, len(other))
	}
	for c, a := range other {
		sum := t[c]
		sum.Usage += a.Usage
		sum.Size += a.Size
		t[c] = sum
	}
	return t
}

// Result holds the classification of one tree. Items are keyed by identity, so
// a Result goes stale once the tree changes, except for removals reported via
// Forget. It is safe for concurrent use.
type Result struct {
	mu      sync.RWMutex
	matches map[fs.Item]Match
	totals  map[fs.Item]Totals
}

// Match returns the classification of an item. Items inside a matched
// directory inherit its classification, with Inherited set, so they still
// show as candidates once the user opens the directory.
func (r *Result) Match(it fs.Item) (Match, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.matchLocked(it)
}

func (r *Result) matchLocked(it fs.Item) (Match, bool) {
	if m, ok := r.matches[it]; ok {
		return m, true
	}
	for p := it.GetParent(); p != nil; p = p.GetParent() {
		if m, ok := r.matches[p]; ok {
			m.Inherited = true
			return m, true
		}
	}
	return Match{}, false
}

// Totals returns the space taken by each category within the item, including
// the item itself when it is a match. It is empty for items with no
// candidates. The returned map must not be modified.
func (r *Result) Totals(it fs.Item) Totals {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalsLocked(it)
}

// Forget updates the result for it having been removed from the tree: its
// space is subtracted from every ancestor, which is far cheaper than
// classifying the whole tree again. Ancestors are found through GetParent, so
// the removed item must keep its parent link.
func (r *Result) Forget(it fs.Item) {
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := r.totalsLocked(it)
	// entries of its descendants are unreachable from now on and left in place
	delete(r.matches, it)
	delete(r.totals, it)
	if len(removed) == 0 {
		return
	}
	for p := it.GetParent(); p != nil; p = p.GetParent() {
		old := r.totals[p]
		// copy, since earlier callers of Totals may still hold the old map
		left := make(Totals, len(old))
		for c, a := range old {
			sub := removed[c]
			a.Usage -= sub.Usage
			a.Size -= sub.Size
			if a.Usage > 0 || a.Size > 0 {
				left[c] = a
			}
		}
		if len(left) == 0 {
			delete(r.totals, p)
		} else {
			r.totals[p] = left
		}
	}
}

func (r *Result) totalsLocked(it fs.Item) Totals {
	if t, ok := r.totals[it]; ok {
		return t
	}
	// a directory holding candidates has an entry, so only matched items
	// (and their descendants) get here with a match
	if m, ok := r.matchLocked(it); ok {
		return Totals{m.Category: {Usage: it.GetUsage(), Size: it.GetSize()}}
	}
	return nil
}

// Analyze classifies the whole tree under root and aggregates the space of
// every category per directory, including items too small for a frontend to
// show.
func (c *Classifier) Analyze(root fs.Item) *Result {
	w := &walker{
		c: c,
		result: &Result{
			matches: make(map[fs.Item]Match),
			totals:  make(map[fs.Item]Totals),
		},
	}
	w.walk(root, root.GetPath(), nil)
	return w.result
}

type walker struct {
	c      *Classifier
	result *Result
	repos  []*repo
}

func (w *walker) walk(it fs.Item, path string, siblings map[string]struct{}) Totals {
	if w.c.excluded(path) {
		return nil
	}
	if m, ok := w.classify(it, path, siblings); ok {
		w.result.matches[it] = m
		return w.result.totalsLocked(it)
	}
	if !it.IsDir() {
		return nil
	}

	var children []fs.Item
	names := make(map[string]struct{})
	for child := range it.GetFilesLocked(fs.SortByName, fs.SortAsc) {
		children = append(children, child)
		names[child.GetName()] = struct{}{}
	}

	popRepo := w.enterDir(path, names)
	defer popRepo()

	// children of the virtual top level dir are scanned roots: they carry
	// their own absolute path, and the virtual dir has none
	virtual := analyze.IsVirtualRootDir(it)
	var totals Totals
	for _, child := range children {
		childPath := filepath.Join(path, child.GetName())
		if virtual {
			childPath = child.GetPath()
		}
		totals = totals.add(w.walk(child, childPath, names))
	}
	if len(totals) > 0 {
		w.result.totals[it] = totals
	}
	return totals
}

func (w *walker) classify(it fs.Item, path string, siblings map[string]struct{}) (Match, bool) {
	ignored := w.gitIgnored(path, it.IsDir())
	for i := range w.c.rules {
		r := &w.c.rules[i]
		if !r.matches(it, path, siblings) {
			continue
		}
		m := Match{Category: r.Category, Confidence: r.Confidence, Hint: r.Hint}
		if ignored {
			// git-ignored output of a recognized tool is regenerable
			m.Confidence = Safe
		}
		return m, true
	}
	if ignored {
		return Match{Category: GitIgnored, Confidence: Review, Hint: "Ignored by .gitignore; may still be local data"}, true
	}
	return Match{}, false
}

func (r *Rule) matches(it fs.Item, path string, siblings map[string]struct{}) bool {
	name := it.GetName()
	switch {
	case r.Dir != "":
		if !it.IsDir() || !globMatch(r.Dir, name) {
			return false
		}
	case r.File != "":
		if it.IsDir() || !globMatch(r.File, name) {
			return false
		}
	default:
		if !globMatch(r.Path, path) {
			return false
		}
	}
	if r.Under != "" && !isBelow(path, r.Under) {
		return false
	}
	if len(r.Markers) == 0 {
		return true
	}
	for _, marker := range r.Markers {
		if _, ok := siblings[marker]; ok {
			return true
		}
		if !strings.ContainsAny(marker, "*?[") {
			continue
		}
		for sibling := range siblings {
			if globMatch(marker, sibling) {
				return true
			}
		}
	}
	return false
}

func globMatch(pattern, name string) bool {
	ok, err := filepath.Match(pattern, name)
	return err == nil && ok
}

func (c *Classifier) excluded(path string) bool {
	for _, ex := range c.excludes {
		if path == ex || isBelow(path, ex) {
			return true
		}
	}
	return false
}

// isBelow reports whether path is strictly inside dir.
func isBelow(path, dir string) bool {
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}
