package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

const home = "/home/u"

// node describes a test tree: a file when children is nil, a directory
// otherwise. A directory's usage is the sum of its children.
type node struct {
	name     string
	usage    int64
	children []node
}

func dir(name string, children ...node) node { return node{name: name, children: children} }
func file(name string, usage int64) node     { return node{name: name, usage: usage} }

// build creates an in-memory tree rooted at basePath/root.name.
func build(basePath string, root node) *analyze.Dir {
	d := &analyze.Dir{File: &analyze.File{Name: root.name}, BasePath: basePath}
	fill(d, root.children)
	return d
}

func fill(d *analyze.Dir, children []node) {
	for _, c := range children {
		var item fs.Item
		if c.children != nil {
			sub := &analyze.Dir{File: &analyze.File{Name: c.name, Parent: d}}
			fill(sub, c.children)
			item = sub
		} else {
			item = &analyze.File{Name: c.name, Usage: c.usage, Size: c.usage, Parent: d}
		}
		d.Files = append(d.Files, item)
		d.Usage += item.GetUsage()
		d.Size += item.GetSize()
	}
}

func child(t *testing.T, it fs.Item, names ...string) fs.Item {
	t.Helper()
	for _, name := range names {
		var found fs.Item
		for c := range it.GetFiles(fs.SortByName, fs.SortAsc) {
			if c.GetName() == name {
				found = c
			}
		}
		require.NotNil(t, found, "child %q of %s", name, it.GetPath())
		it = found
	}
	return it
}

func newClassifier(t *testing.T, cfg Config) *Classifier {
	t.Helper()
	c, err := NewClassifier(cfg, home)
	require.NoError(t, err)
	return c
}

func TestMarkerRequired(t *testing.T) {
	root := build("/home", dir("u",
		dir("app", file("package.json", 1), dir("node_modules", file("x.js", 100))),
		dir("loose", dir("node_modules", file("y.js", 50))),
	))
	res := newClassifier(t, Config{}).Analyze(root)

	m, ok := res.Match(child(t, root, "app", "node_modules"))
	require.True(t, ok)
	assert.Equal(t, BuildArtifact, m.Category)
	assert.Equal(t, Safe, m.Confidence)

	_, ok = res.Match(child(t, root, "loose", "node_modules"))
	assert.False(t, ok, "node_modules without package.json is not a candidate")
}

func TestUnitMatchHidesDescendants(t *testing.T) {
	root := build("/home", dir("u",
		dir("app", file("package.json", 1), dir("node_modules", dir("pkg", file("model.gguf", 10)))),
	))
	res := newClassifier(t, Config{}).Analyze(root)

	_, ok := res.Match(child(t, root, "app", "node_modules", "pkg", "model.gguf"))
	assert.False(t, ok)
	assert.Equal(t, Totals{BuildArtifact: {Usage: 10, Size: 10}}, res.Totals(child(t, root, "app")))
}

func TestPathRulesAndPrecedence(t *testing.T) {
	root := build("/home", dir("u",
		dir(".cache", dir("huggingface", file("w", 30)), dir("thumbs", file("t", 5))),
		dir("Downloads", file("a.zip", 7), file("setup.dmg", 9)),
		dir("src", file("b.zip", 3)),
	))
	res := newClassifier(t, Config{}).Analyze(root)

	tests := []struct {
		path     []string
		category Category
		matched  bool
	}{
		{[]string{".cache", "huggingface"}, MLModel, true},
		{[]string{".cache", "thumbs"}, AppCache, true},
		{[]string{"Downloads", "a.zip"}, Installer, true},
		{[]string{"Downloads", "setup.dmg"}, Installer, true},
		{[]string{"src", "b.zip"}, "", false},
	}
	for _, tt := range tests {
		m, ok := res.Match(child(t, root, tt.path...))
		assert.Equal(t, tt.matched, ok, tt.path)
		assert.Equal(t, tt.category, m.Category, tt.path)
	}
	_, ok := res.Match(child(t, root, ".cache"))
	assert.False(t, ok, "~/.cache itself is not a unit, its children are")
}

func TestTotalsAggregateMixedDirectory(t *testing.T) {
	root := build("/home", dir("u",
		dir("p", file("package.json", 1), dir("node_modules", file("x", 100)), file("notes.txt", 20)),
		dir("m", file("a.gguf", 40), file("b.safetensors", 60)),
		file("doc.txt", 5),
	))
	res := newClassifier(t, Config{}).Analyze(root)

	assert.Equal(t, Totals{
		BuildArtifact: {Usage: 100, Size: 100},
		MLModel:       {Usage: 100, Size: 100},
	}, res.Totals(root))
	assert.Equal(t, Totals{MLModel: {Usage: 40, Size: 40}}, res.Totals(child(t, root, "m", "a.gguf")))
	assert.Nil(t, res.Totals(child(t, root, "doc.txt")))
}

func TestExclusionsAndUserRules(t *testing.T) {
	root := build("/home", dir("u",
		dir("work", file("package.json", 1), dir("node_modules", file("x", 100))),
		dir("data", dir("weights", file("w", 10))),
	))
	res := newClassifier(t, Config{
		Exclude: []string{"~/work"},
		Rules:   []Rule{{Category: MLModel, Dir: "weights", Hint: "mine"}},
	}).Analyze(root)

	_, ok := res.Match(child(t, root, "work", "node_modules"))
	assert.False(t, ok)

	m, ok := res.Match(child(t, root, "data", "weights"))
	require.True(t, ok)
	assert.Equal(t, Match{Category: MLModel, Confidence: Review, Hint: "mine"}, m)
}

func TestInvalidRules(t *testing.T) {
	tests := map[string]Rule{
		"unknown category":   {Category: "junk", Dir: "x"},
		"unknown confidence": {Category: MLModel, Confidence: "maybe", Dir: "x"},
		"no matcher":         {Category: MLModel},
		"two matchers":       {Category: MLModel, Dir: "x", File: "y"},
		"bad glob":           {Category: MLModel, File: "["},
	}
	for name, rule := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewClassifier(Config{Rules: []Rule{rule}}, home)
			assert.Error(t, err)
		})
	}
}

func TestVirtualRootChildrenUseOwnPaths(t *testing.T) {
	a := build("/home/u", dir("a", file("package.json", 1), dir("node_modules", file("x", 4))))
	b := build("/home/u", dir("b", file("big.iso", 8)))
	root := analyze.CreateVirtualRootDir(a, b)
	res := newClassifier(t, Config{}).Analyze(root)

	assert.Equal(t, Totals{
		BuildArtifact: {Usage: 4, Size: 4},
		Installer:     {Usage: 8, Size: 8},
	}, res.Totals(root))
}

func TestGitIgnoreSignal(t *testing.T) {
	base := t.TempDir()
	repoDir := filepath.Join(base, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("# c\nout/\n*.log\n!keep.log\ndist\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "sub", ".gitignore"), []byte("local/\n"), 0o644))

	root := build(base, dir("repo",
		dir(".git", file("HEAD", 1)),
		file(".gitignore", 1),
		file("package.json", 1),
		dir("out", file("o", 10)),
		file("app.log", 2),
		file("keep.log", 3),
		dir("dist", file("d", 4)),
		dir("sub", file(".gitignore", 1), dir("local", file("l", 5))),
		dir("src", file("main.go", 6)),
	))
	res := newClassifier(t, Config{}).Analyze(root)

	tests := []struct {
		path       []string
		matched    bool
		category   Category
		confidence Confidence
	}{
		{[]string{"out"}, true, GitIgnored, Review},
		{[]string{"app.log"}, true, GitIgnored, Review},
		{[]string{"keep.log"}, false, "", ""},
		// a rule match that is also git-ignored is upgraded to safe
		{[]string{"dist"}, true, BuildArtifact, Safe},
		{[]string{"sub", "local"}, true, GitIgnored, Review},
		{[]string{"src"}, false, "", ""},
		{[]string{".git"}, false, "", ""},
	}
	for _, tt := range tests {
		m, ok := res.Match(child(t, root, tt.path...))
		assert.Equal(t, tt.matched, ok, tt.path)
		assert.Equal(t, tt.category, m.Category, tt.path)
		assert.Equal(t, tt.confidence, m.Confidence, tt.path)
	}
}

func TestNoRepoNoGitIgnoreSignal(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "plain"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "plain", ".gitignore"), []byte("out/\n"), 0o644))

	root := build(base, dir("plain", file(".gitignore", 1), dir("out", file("o", 10))))
	res := newClassifier(t, Config{}).Analyze(root)

	_, ok := res.Match(child(t, root, "out"))
	assert.False(t, ok)
}

func TestForgetSubtractsFromAncestors(t *testing.T) {
	root := build("/home", dir("u",
		dir("p", file("package.json", 1), dir("node_modules", file("x", 100)), file("a.gguf", 40)),
		file("b.gguf", 60),
	))
	res := newClassifier(t, Config{}).Analyze(root)
	before := res.Totals(root)

	modules := child(t, root, "p", "node_modules")
	res.Forget(modules)

	_, ok := res.Match(modules)
	assert.False(t, ok)
	assert.Equal(t, Totals{MLModel: {Usage: 40, Size: 40}}, res.Totals(child(t, root, "p")))
	assert.Equal(t, Totals{MLModel: {Usage: 100, Size: 100}}, res.Totals(root))
	// maps handed out earlier are not modified
	assert.Equal(t, int64(100), before[BuildArtifact].Usage)

	res.Forget(child(t, root, "p", "a.gguf"))
	assert.Nil(t, res.Totals(child(t, root, "p")))

	// forgetting an item without candidates changes nothing
	res.Forget(child(t, root, "p", "package.json"))
	assert.Equal(t, Totals{MLModel: {Usage: 60, Size: 60}}, res.Totals(root))
}

func TestPathRulesUnderVirtualRoot(t *testing.T) {
	// scanning several paths puts them under the virtual top level dir, whose
	// own name must not leak into the paths path rules are matched against
	goDir := build(home, dir("go", dir("pkg", dir("mod", file("m", 30)))))
	gradle := build(home, dir(".gradle", dir("caches", file("c", 20))))
	root := analyze.CreateVirtualRootDir(goDir, gradle)
	res := newClassifier(t, Config{}).Analyze(root)

	assert.Equal(t, Totals{PackageCache: {Usage: 50, Size: 50}}, res.Totals(root))
	m, ok := res.Match(child(t, goDir, "pkg", "mod"))
	require.True(t, ok)
	assert.Equal(t, PackageCache, m.Category)
}

func TestDefaultCacheAndToolchainRules(t *testing.T) {
	root := build("/home", dir("u",
		dir(".cache", dir("uv", file("u", 1)), dir("thumbs", file("t", 1))),
		dir(".gradle", dir("wrapper", dir("dists", file("d", 1))), dir("caches", file("c", 1))),
		dir(".rustup", dir("toolchains", file("r", 1))),
		dir(".bun", dir("install", dir("global", file("package.json", 1), dir("node_modules", file("n", 1))))),
		dir("Library", dir("Caches", dir("go-build", file("g", 1)))),
		dir("app", dir("App.xcodeproj", file("p", 1)), dir("DerivedData", file("d", 1))),
	))
	res := newClassifier(t, Config{}).Analyze(root)

	tests := []struct {
		path     []string
		category Category
	}{
		{[]string{".cache", "uv"}, PackageCache},
		{[]string{".cache", "thumbs"}, AppCache},
		{[]string{".gradle", "wrapper", "dists"}, PackageCache},
		{[]string{".gradle", "caches"}, PackageCache},
		{[]string{".rustup", "toolchains"}, Toolchain},
		// global installs are not build artifacts despite their package.json
		{[]string{".bun", "install", "global"}, Toolchain},
		{[]string{"Library", "Caches", "go-build"}, PackageCache},
		// glob marker: next to *.xcodeproj
		{[]string{"app", "DerivedData"}, BuildArtifact},
	}
	for _, tt := range tests {
		m, ok := res.Match(child(t, root, tt.path...))
		assert.True(t, ok, tt.path)
		assert.Equal(t, tt.category, m.Category, tt.path)
	}
}
