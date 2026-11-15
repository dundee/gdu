package common_test

import (
	"github.com/dundee/gdu/v6/internal/common"
	"github.com/dundee/gdu/v6/internal/config"
)

// testUI builds a common.UI step by step for the tests of leaving things out
// of a scan: each setter adds to the settings, compiles the ignore patterns of
// all their sources as gdu does, and returns the error doing so.
type testUI struct {
	*common.UI
	scan      config.Scan
	patterns  []string
	file      string
	gitignore string
	roots     []string
}

func newTestUI() *testUI {
	return &testUI{UI: common.NewUI(nil, &config.Settings{})}
}

func (ui *testUI) build() error {
	pattern, err := common.IgnorePattern(ui.patterns, ui.file, ui.gitignore, ui.roots)
	if err != nil {
		return err
	}
	ui.scan.IgnoreDirPattern = pattern
	ui.UI = common.NewUI(nil, &config.Settings{Scan: ui.scan})
	return nil
}

func (ui *testUI) SetIgnoreDirPaths(paths []string) {
	ui.scan.IgnoreDirs = paths
	_ = ui.build()
}

func (ui *testUI) SetIgnoreHidden(value bool) {
	ui.scan.IgnoreHidden = value
	_ = ui.build()
}

func (ui *testUI) SetIgnoreTypes(types []string) {
	ui.scan.ExcludeTypes = types
	_ = ui.build()
}

func (ui *testUI) SetIncludeTypes(types []string) {
	ui.scan.IncludeTypes = types
	_ = ui.build()
}

func (ui *testUI) SetIgnoreDirPatterns(patterns []string) error {
	ui.patterns = append(ui.patterns, patterns...)
	return ui.build()
}

func (ui *testUI) SetIgnoreFromFile(file string) error {
	ui.file = file
	return ui.build()
}

func (ui *testUI) SetIgnoreFromGitignoreFile(file string, roots []string) error {
	ui.gitignore, ui.roots = file, roots
	return ui.build()
}
