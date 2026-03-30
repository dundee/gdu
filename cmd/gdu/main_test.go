package main

import "testing"

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
