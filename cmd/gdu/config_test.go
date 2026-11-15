package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/dundee/gdu/v6/cmd/gdu/app"
)

// withConfig runs initConfig with the system config system and the user
// config user, each the content of a file or absent when nil, and returns
// what it reports.
func withConfig(t *testing.T, system, user []byte, args ...string) string {
	t.Helper()
	dir := t.TempDir()
	systemPath := filepath.Join(dir, "system.yaml")
	userPath := filepath.Join(dir, "user.yaml")
	if system != nil {
		require.NoError(t, os.WriteFile(systemPath, system, 0o600))
	}
	if user != nil {
		require.NoError(t, os.WriteFile(userPath, user, 0o600))
	}

	origArgs, origPath, origErr, origAf := os.Args, systemConfigPath, configErr, af
	origWarnings, origBaseline, origUser := configWarnings, baseline, userConfig
	t.Cleanup(func() {
		os.Args, systemConfigPath, configErr, af = origArgs, origPath, origErr, origAf
		configWarnings, baseline, userConfig = origWarnings, origBaseline, origUser
	})
	systemConfigPath = systemPath
	if len(args) == 0 {
		args = []string{"--config-file=" + userPath}
	}
	os.Args = append([]string{"gdu"}, args...)
	af = &app.Flags{LogFile: "/dev/null"}
	configErr, configWarnings, baseline, userConfig = nil, nil, nil, nil

	initConfig()
	setDefaults()
	var stderr bytes.Buffer
	reportConfigError(&stderr)
	return stderr.String()
}

// A misspelt setting is told, not silently ignored: it may be a safety one.
func TestUnknownConfigKeysAreReported(t *testing.T) {
	out := withConfig(t, nil, []byte("no-color: true\nno-delte: true\n"))

	assert.Contains(t, out, "ignoring unknown settings: no-delte (line 2)")
	assert.True(t, af.NoColor, "the known settings still apply")
}

func TestMissingExplicitConfigFileIsReported(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")

	out := withConfig(t, nil, nil, "--config-file", missing)

	assert.Contains(t, out, "gdu: ignoring the config file: "+missing)
}

func TestMissingDefaultConfigFileIsFine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	out := withConfig(t, nil, nil, "-n")

	assert.Empty(t, out)
}

// A broken system config does not keep the user's from applying, and changes
// nothing itself, though part of it parses.
func TestBrokenSystemConfigStillLoadsUserConfig(t *testing.T) {
	out := withConfig(t, []byte("no-delete: true\nno-color: [\n"), []byte("show-apparent-size: true\n"))

	assert.Contains(t, out, "system.yaml")
	assert.True(t, af.ShowApparentSize)
	assert.False(t, af.NoDelete, "the broken file applied nothing")
}

// --write-config saves what the user configured: the config file and what the
// command line changed, but neither the defaults nor the system config, and
// not the one-off settings of the command line.
func TestWriteConfigSavesWhatTheUserConfigured(t *testing.T) {
	withConfig(t, []byte("no-mouse: true\nshow-relative-size: true\n"), []byte("output-file: kept.json\nno-color: true\n"))
	// as the command line would
	af.ShowApparentSize = true
	af.InputFile = "in.json"
	af.DbPath = "x.badger"
	af.MaxAge = "7d"

	var saved map[string]any
	data, err := yaml.Marshal(configToSave(configNode(af), baseline, userConfig))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &saved))

	assert.Equal(t, map[string]any{
		"output-file":        "kept.json",
		"no-color":           true,
		"show-apparent-size": true,
	}, saved)
}

func TestWriteConfigOfNothingIsAnEmptyMapping(t *testing.T) {
	withConfig(t, nil, nil)

	data, err := yaml.Marshal(configToSave(configNode(af), baseline, userConfig))

	require.NoError(t, err)
	assert.Equal(t, "{}\n", string(data))
}
