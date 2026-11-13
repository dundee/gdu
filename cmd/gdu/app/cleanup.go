package app

import "github.com/dundee/gdu/v5/internal/cleanup"

// CleanupConfig is the cleanup section of the configuration file: the user's
// rules and exclusions for the web UI's cleanup candidates.
type CleanupConfig struct {
	// Exclude lists paths that are never candidates, nor anything below them.
	Exclude []string `yaml:"exclude"`
	// Rules are evaluated before the built-in rules.
	Rules []CleanupRule `yaml:"rules"`
}

// CleanupRule marks the items it matches as cleanup candidates. Exactly one
// of Dir, File and Path is set.
type CleanupRule struct {
	// Category is one of the categories of the web UI, such as "cache" or
	// "build".
	Category string `yaml:"category"`
	// Confidence is "safe" or "review"; empty means "review".
	Confidence string `yaml:"confidence"`
	// Dir is a glob matched against the base name of directories.
	Dir string `yaml:"dir"`
	// File is a glob matched against the base name of files.
	File string `yaml:"file"`
	// Path is a glob matched against the absolute path of any item; a
	// leading "~/" stands for the home directory.
	Path string `yaml:"path"`
	// Under restricts the rule to items below this directory ("~/" expands).
	Under string `yaml:"under"`
	// Markers, when set, require at least one sibling with one of these names,
	// e.g. node_modules only counts as a build artifact next to package.json.
	Markers []string `yaml:"markers"`
	// Hint is shown to the user next to the candidate.
	Hint string `yaml:"hint"`
}

// config returns the configuration the classifier takes.
func (c CleanupConfig) config() cleanup.Config {
	cfg := cleanup.Config{Exclude: c.Exclude}
	for i := range c.Rules {
		r := &c.Rules[i]
		cfg.Rules = append(cfg.Rules, cleanup.Rule{
			Category:   cleanup.Category(r.Category),
			Confidence: cleanup.Confidence(r.Confidence),
			Dir:        r.Dir,
			File:       r.File,
			Path:       r.Path,
			Under:      r.Under,
			Markers:    r.Markers,
			Hint:       r.Hint,
		})
	}
	return cfg
}
