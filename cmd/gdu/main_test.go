package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/cmd/gdu/app"
)

func TestNoViewFileFlagRegistered(t *testing.T) {
	flag := rootCmd.Flags().Lookup("no-view-file")
	if flag == nil {
		t.Fatal("expected no-view-file flag to be registered")
	}
}

func TestNoViewFileFlagCanBeSet(t *testing.T) {
	t.Cleanup(func() {
		_ = rootCmd.Flags().Set("no-view-file", "false")
	})

	err := rootCmd.Flags().Set("no-view-file", "true")
	if err != nil {
		t.Fatalf("expected setting no-view-file flag to succeed: %v", err)
	}

	if !af.NoViewFile {
		t.Fatal("expected NoViewFile to be true after setting flag")
	}
}

func TestInteractiveFlagRegistered(t *testing.T) {
	flag := rootCmd.Flags().Lookup("interactive")
	if flag == nil {
		t.Fatal("expected interactive flag to be registered")
	}
}

func TestInteractiveFlagCanBeSet(t *testing.T) {
	t.Cleanup(func() {
		_ = rootCmd.Flags().Set("interactive", "false")
	})

	err := rootCmd.Flags().Set("interactive", "true")
	if err != nil {
		t.Fatalf("expected setting interactive flag to succeed: %v", err)
	}

	if !af.Interactive {
		t.Fatal("expected Interactive to be true after setting flag")
	}
}

func TestInitConfigMalformedSystemConfig(t *testing.T) {
	// Write invalid YAML to a temp file and point systemConfigPath at it.
	tmp := filepath.Join(t.TempDir(), "gdu.yaml")
	if err := os.WriteFile(tmp, []byte(":\tinvalid: yaml: {"), 0o600); err != nil {
		t.Fatalf("could not write temp config: %v", err)
	}

	origPath := systemConfigPath
	origErr := configErr
	origAf := af
	t.Cleanup(func() {
		systemConfigPath = origPath
		configErr = origErr
		af = origAf
	})

	systemConfigPath = tmp
	af = &app.Flags{}
	configErr = nil

	initConfig()

	if configErr == nil {
		t.Fatal("expected configErr to be set for malformed system config, got nil")
	}
}

func TestInitConfigMalformedUserConfig(t *testing.T) {
	// Write invalid YAML to a temp file and pass it via --config-file.
	tmp := filepath.Join(t.TempDir(), "user.yaml")
	if err := os.WriteFile(tmp, []byte(":\tinvalid: yaml: {"), 0o600); err != nil {
		t.Fatalf("could not write temp config: %v", err)
	}

	origArgs := os.Args
	origPath := systemConfigPath
	origErr := configErr
	origAf := af
	t.Cleanup(func() {
		os.Args = origArgs
		systemConfigPath = origPath
		configErr = origErr
		af = origAf
	})

	// Point system config at a nonexistent path so it is skipped cleanly.
	systemConfigPath = filepath.Join(t.TempDir(), "nonexistent.yaml")
	os.Args = []string{"gdu", "--config-file=" + tmp}
	af = &app.Flags{}
	configErr = nil

	initConfig()

	if configErr == nil {
		t.Fatal("expected configErr to be set for malformed user config, got nil")
	}
}
